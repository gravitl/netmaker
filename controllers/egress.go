package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func egressHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/egress/presets", logic.SecurityCheck(true, http.HandlerFunc(getEgressPresets))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/egress", logic.SecurityCheck(true, http.HandlerFunc(createEgress))).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/egress", logic.SecurityCheck(true, http.HandlerFunc(listEgress))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/egress", logic.SecurityCheck(true, http.HandlerFunc(updateEgress))).Methods(http.MethodPut)
	r.HandleFunc("/api/v1/egress", logic.SecurityCheck(true, http.HandlerFunc(deleteEgress))).Methods(http.MethodDelete)
}

// @Summary     List egress domain presets
// @Router      /api/v1/egress/presets [get]
// @Tags        Egress
// @Security    oauth
// @Produce     json
// @Success     200 {object} models.SuccessResponse
// @Failure     401 {object} models.ErrorResponse
func getEgressPresets(w http.ResponseWriter, r *http.Request) {
	presets := logic.ListEgressPresets()
	logic.ReturnSuccessResponseWithJson(w, r, map[string]any{
		"presetApps": presets,
	}, "fetched egress preset catalog")
}

// @Summary     Create Egress Resource
// @Router      /api/v1/egress [post]
// @Tags        Egress
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       body body models.EgressReq true "Egress request data"
// @Success     200 {object} schema.Egress
// @Failure     400 {object} models.ErrorResponse
// @Failure     401 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func createEgress(w http.ResponseWriter, r *http.Request) {

	var req models.EgressReq
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logger.Log(0, "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if req.PresetID != "" {
		if err := logic.ApplyEgressPresetToEgressReq(&req); err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
	}
	normDomains, err := logic.NormalizeEgressReqDomains(req.Domains)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if req.IsInetGw {
		normDomains = nil
	}
	req.Domains = normDomains
	var resolvedCIDRs []string
	if req.PresetID != "" {
		if p, ok := logic.GetEgressPresetByID(req.PresetID); ok && logic.PresetYieldsAWSIPRanges(p) {
			if c, err := logic.ResolveAWSEgressPresetCIDRs(http.DefaultClient, p); err == nil && len(c) > 0 {
				resolvedCIDRs = c
			} else if err != nil {
				logger.Log(0, "aws preset ip fetch failed:", req.PresetID, err.Error())
			}
		}
	}
	var egressRange string
	if !req.IsInetGw {
		if len(normDomains) > 0 {
			egressRange = ""
		} else if req.Range != "" {
			egressRange, err = logic.NormalizeCIDR(req.Range)
			if err != nil {
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
				return
			}
		}
	} else {
		egressRange = "*"
		req.Domains = nil
	}
	network := &schema.Network{Name: req.Network}
	err = network.Get(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	e := schema.Egress{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Network:     req.Network,
		Description: req.Description,
		Range:       egressRange,
		Nat:         req.Nat,
		Mode:        req.Mode,
		Nodes:       make(datatypes.JSONMap),
		Tags:        make(datatypes.JSONMap),
		PresetID:    req.PresetID,
		Status:      true,
		CreatedBy:   r.Header.Get("user"),
		CreatedAt:   time.Now().UTC(),
	}
	logic.ApplyConfiguredDomainsToEgress(&e, normDomains)
	if len(resolvedCIDRs) > 0 {
		logic.SetEgressDomainAnsForDomains(&e, normDomains, resolvedCIDRs)
	}
	if len(req.Tags) > 0 {
		for tagID, metric := range req.Tags {
			e.Tags[tagID] = metric
		}
		e.Nodes = make(datatypes.JSONMap)
	} else {
		for nodeID, metric := range req.Nodes {
			e.Nodes[nodeID] = metric
		}
	}
	if err := logic.ValidateEgressReq(&e); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if err := logic.AssignVirtualRangeToEgress(network, &e); err != nil {
		logger.Log(0, "error assigning virtual range to egress: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logger.Log(1, fmt.Sprintf("createEgress: after AssignVirtualRangeToEgress, e.VirtualRange = '%s', e.Mode = '%s', e.Nat = %v", e.VirtualRange, e.Mode, e.Nat))
	err = e.Create(db.WithContext(r.Context()))
	if err != nil {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("error creating egress resource"+err.Error()), "internal"),
		)
		return
	}
	logic.LogEvent(&models.Event{
		Action: schema.Create,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   e.ID,
			Name: e.Name,
			Type: schema.EgressSub,
		},
		NetworkID: schema.NetworkID(e.Network),
		Origin:    schema.Dashboard,
	})
	// for nodeID := range e.Nodes {
	// 	node, err := logic.GetNodeByID(nodeID)
	// 	if err != nil {
	// 		logic.AddEgressInfoToNode(&node, e)
	// 		logic.UpsertNode(&node)
	// 	}

	// }
	if len(normDomains) > 0 && !logic.HasEgressDomainAns(e) {
		if req.Nodes != nil {
			for nodeID := range req.Nodes {
				node, err := logic.GetNodeByID(nodeID)
				if err != nil {
					continue
				}
				host := &schema.Host{
					ID: node.HostID,
				}
				err = host.Get(r.Context())
				if err != nil {
					continue
				}
				for _, dom := range normDomains {
					mq.HostUpdate(&models.HostUpdate{
						Action: models.EgressUpdate,
						Host:   *host,
						EgressDomain: models.EgressDomain{
							ID:     e.ID,
							Host:   *host,
							Node:   node,
							Domain: dom,
						},
						Node: node,
					})
				}
			}
		}
	} else {
		go mq.PublishPeerUpdate(false)
	}

	logic.ReturnSuccessResponseWithJson(w, r, e, "created egress resource")
}

// @Summary     List Egress Resources
// @Router      /api/v1/egress [get]
// @Tags        Egress
// @Security    oauth
// @Produce     json
// @Param       network query string true "Network identifier"
// @Success     200 {array} schema.Egress
// @Failure     400 {object} models.ErrorResponse
// @Failure     401 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func listEgress(w http.ResponseWriter, r *http.Request) {

	network := r.URL.Query().Get("network")
	if network == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("network is required"), "badrequest"))
		return
	}
	e := schema.Egress{Network: network}
	list, err := e.ListByNetwork(db.WithContext(r.Context()))
	if err != nil {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("error listing egress resource"+err.Error()), "internal"),
		)
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, list, "fetched egress resource list")
}

// @Summary     Update Egress Resource
// @Router      /api/v1/egress [put]
// @Tags        Egress
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       body body models.EgressReq true "Egress request data"
// @Success     200 {object} schema.Egress
// @Failure     400 {object} models.ErrorResponse
// @Failure     401 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func updateEgress(w http.ResponseWriter, r *http.Request) {

	var req models.EgressReq
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logger.Log(0, "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if req.PresetID != "" {
		if err := logic.ApplyEgressPresetToEgressReq(&req); err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
	}
	normDomains, err := logic.NormalizeEgressReqDomains(req.Domains)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if req.IsInetGw {
		normDomains = nil
	}
	req.Domains = normDomains
	network := &schema.Network{Name: req.Network}
	err = network.Get(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	var egressRange string
	if !req.IsInetGw {
		if len(normDomains) > 0 {
			egressRange = ""
		} else if req.Range != "" {
			egressRange, err = logic.NormalizeCIDR(req.Range)
			if err != nil {
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
				return
			}
		}
	} else {
		egressRange = "*"
		req.Domains = nil
	}

	e := schema.Egress{ID: req.ID}
	err = e.Get(db.WithContext(r.Context()))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	oldConfigured := logic.ConfiguredDomainsForEgress(e)
	oldPresetID := e.PresetID
	oldMode := e.Mode

	e.Range = egressRange
	event := &models.Event{
		Action: schema.Update,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   e.ID,
			Name: e.Name,
			Type: schema.EgressSub,
		},
		Diff: models.Diff{
			Old: e,
		},
		NetworkID: schema.NetworkID(e.Network),
		Origin:    schema.Dashboard,
	}
	e.Nodes = make(datatypes.JSONMap)
	e.Tags = make(datatypes.JSONMap)
	if len(req.Tags) > 0 {
		for tagID, metric := range req.Tags {
			e.Tags[tagID] = metric
		}
		e.Nodes = make(datatypes.JSONMap)
	} else {
		for nodeID, metric := range req.Nodes {
			e.Nodes[nodeID] = metric
		}
	}
	if !logic.EgressDomainsEqual(oldConfigured, normDomains) {
		logic.ClearEgressDomainAns(&e)
	}
	e.Description = req.Description
	e.Name = req.Name
	logic.ApplyConfiguredDomainsToEgress(&e, normDomains)
	e.Status = req.Status
	if req.PresetID != "" {
		e.PresetID = req.PresetID
	}
	if !req.Nat {
		e.Nat = false
		e.Mode = schema.DisabledNAT
		e.VirtualRange = ""
	} else if req.Mode == schema.VirtualNAT {
		e.Nat = true
		e.Mode = schema.VirtualNAT
	} else {
		e.Nat = true
		e.Mode = schema.DirectNAT
		e.VirtualRange = ""
	}
	presetChanged := req.PresetID != "" && req.PresetID != oldPresetID
	domainsChanged := !logic.EgressDomainsEqual(oldConfigured, normDomains)
	if req.PresetID != "" && (presetChanged || domainsChanged) {
		if p, ok := logic.GetEgressPresetByID(req.PresetID); ok && logic.PresetYieldsAWSIPRanges(p) {
			if c, err := logic.ResolveAWSEgressPresetCIDRs(http.DefaultClient, p); err == nil && len(c) > 0 {
				logic.SetEgressDomainAnsForDomains(&e, normDomains, c)
			} else if err != nil {
				logger.Log(0, "aws preset ip fetch failed:", req.PresetID, err.Error())
			}
		}
	}
	e.UpdatedAt = time.Now().UTC()
	if err := logic.ValidateEgressReq(&e); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if e.Nat && e.Mode == schema.VirtualNAT {
		if (oldMode != schema.VirtualNAT) || (e.VirtualRange == "") {
			if err := logic.AssignVirtualRangeToEgress(network, &e); err != nil {
				logger.Log(0, "error assigning virtual range to egress: ", err.Error())
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
				return
			}
		}
	}

	// Build update map with all fields including zero values
	// GORM's Updates(&e) doesn't update zero values, so we use a map explicitly
	updateMap := map[string]any{
		"name":                 e.Name,
		"description":          e.Description,
		"range":                e.Range,
		"domains":              e.Domains,
		"nat":                  e.Nat,
		"mode":                 e.Mode,
		"status":               e.Status,
		"nodes":                e.Nodes,
		"tags":                 e.Tags,
		"domain_ans_by_domain": e.DomainAnsByDomain,
		"virtual_range":        e.VirtualRange,
		"preset_id":            e.PresetID,
		"updated_at":           e.UpdatedAt,
	}

	// Perform single update with all fields including zero values
	err = db.FromContext(r.Context()).Table(e.Table()).Where("id = ?", e.ID).Updates(updateMap).Error
	if err != nil {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("error updating egress resource: "+err.Error()), "internal"),
		)
		return
	}
	event.Diff.New = e
	logic.LogEvent(event)
	if len(normDomains) > 0 && !logic.HasEgressDomainAns(e) {
		if req.Nodes != nil {
			for nodeID := range req.Nodes {
				node, err := logic.GetNodeByID(nodeID)
				if err != nil {
					continue
				}
				host := &schema.Host{
					ID: node.HostID,
				}
				err = host.Get(r.Context())
				if err != nil {
					continue
				}
				for _, dom := range normDomains {
					mq.HostUpdate(&models.HostUpdate{
						Action: models.EgressUpdate,
						Host:   *host,
						EgressDomain: models.EgressDomain{
							ID:     e.ID,
							Host:   *host,
							Node:   node,
							Domain: dom,
						},
						Node: node,
					})
				}
			}
		}
	}
	go mq.PublishPeerUpdate(false)
	logic.ReturnSuccessResponseWithJson(w, r, e, "updated egress resource")
}

// @Summary     Delete Egress Resource
// @Router      /api/v1/egress [delete]
// @Tags        Egress
// @Security    oauth
// @Produce     json
// @Param       id query string true "Egress resource ID"
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     401 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func deleteEgress(w http.ResponseWriter, r *http.Request) {

	id := r.URL.Query().Get("id")
	if id == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("id is required"), "badrequest"))
		return
	}
	e := schema.Egress{ID: id}
	err := e.Get(db.WithContext(r.Context()))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}
	err = e.Delete(db.WithContext(r.Context()))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}
	logic.LogEvent(&models.Event{
		Action: schema.Delete,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   e.ID,
			Name: e.Name,
			Type: schema.EgressSub,
		},
		NetworkID: schema.NetworkID(e.Network),
		Origin:    schema.Dashboard,
		Diff: models.Diff{
			Old: e,
			New: nil,
		},
	})
	// delete related acl policies
	acls := logic.ListAcls()
	for _, acl := range acls {

		for i := len(acl.Dst) - 1; i >= 0; i-- {
			if acl.Dst[i].ID == models.EgressID && acl.Dst[i].Value == id {
				acl.Dst = append(acl.Dst[:i], acl.Dst[i+1:]...)
			}
		}
		if len(acl.Dst) == 0 {
			logic.DeleteAcl(acl)
		} else {
			logic.UpsertAcl(acl)
		}
	}
	go mq.PublishPeerUpdate(false)
	logic.ReturnSuccessResponseWithJson(w, r, nil, "deleted egress resource")
}
