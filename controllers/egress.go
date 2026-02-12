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
	r.HandleFunc("/api/v1/egress", logic.SecurityCheck(true, http.HandlerFunc(createEgress))).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/egress", logic.SecurityCheck(true, http.HandlerFunc(listEgress))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/egress", logic.SecurityCheck(true, http.HandlerFunc(updateEgress))).Methods(http.MethodPut)
	r.HandleFunc("/api/v1/egress", logic.SecurityCheck(true, http.HandlerFunc(deleteEgress))).Methods(http.MethodDelete)
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
	var egressRange string
	if !req.IsInetGw {
		if req.Range != "" {
			var err error
			egressRange, err = logic.NormalizeCIDR(req.Range)
			if err != nil {
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
				return
			}
		}

		if req.Domain != "" {
			isDomain := logic.IsFQDN(req.Domain)
			if !isDomain {
				logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("bad domain name"), "badrequest"))
				return
			}

			egressRange = ""
		}
	} else {
		egressRange = "*"
		req.Domain = ""
	}
	network, err := logic.GetNetwork(req.Network)
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
		Domain:      req.Domain,
		DomainAns:   []string{},
		Nat:         req.Nat,
		Mode:        req.Mode,
		Nodes:       make(datatypes.JSONMap),
		Tags:        make(datatypes.JSONMap),
		Status:      true,
		CreatedBy:   r.Header.Get("user"),
		CreatedAt:   time.Now().UTC(),
	}
	if err := logic.AssignVirtualRangeToEgress(&network, &e); err != nil {
		logger.Log(0, "error assigning virtual range to egress: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logger.Log(1, fmt.Sprintf("createEgress: after AssignVirtualRangeToEgress, e.VirtualRange = '%s', e.Mode = '%s', e.Nat = %v", e.VirtualRange, e.Mode, e.Nat))
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
		Action: models.Create,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: models.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   e.ID,
			Name: e.Name,
			Type: models.EgressSub,
		},
		NetworkID: models.NetworkID(e.Network),
		Origin:    models.Dashboard,
	})
	// for nodeID := range e.Nodes {
	// 	node, err := logic.GetNodeByID(nodeID)
	// 	if err != nil {
	// 		logic.AddEgressInfoToNode(&node, e)
	// 		logic.UpsertNode(&node)
	// 	}

	// }
	if req.Domain != "" {
		if req.Nodes != nil {
			for nodeID := range req.Nodes {
				node, err := logic.GetNodeByID(nodeID)
				if err != nil {
					continue
				}
				host, _ := logic.GetHost(node.HostID.String())
				if host == nil {
					continue
				}
				mq.HostUpdate(&models.HostUpdate{
					Action: models.EgressUpdate,
					Host:   *host,
					EgressDomain: models.EgressDomain{
						ID:     e.ID,
						Host:   *host,
						Node:   node,
						Domain: e.Domain,
					},
					Node: node,
				})
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
	network, err := logic.GetNetwork(req.Network)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	var egressRange string
	if !req.IsInetGw {
		if req.Range != "" {
			var err error
			egressRange, err = logic.NormalizeCIDR(req.Range)
			if err != nil {
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
				return
			}
		}

		if req.Domain != "" {
			isDomain := logic.IsFQDN(req.Domain)
			if !isDomain {
				logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("bad domain name"), "badrequest"))
				return
			}

			egressRange = ""
		}
	} else {
		egressRange = "*"
		req.Domain = ""
	}

	e := schema.Egress{ID: req.ID}
	err = e.Get(db.WithContext(r.Context()))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	// Store old mode for comparison (before we modify e)
	oldMode := e.Mode

	// Update Range first so AssignVirtualRangeToEgress can use the correct range
	e.Range = egressRange

	// Update mode and NAT before calling AssignVirtualRangeToEgress
	// This ensures the function sees the new values
	if req.Mode != models.VirtualNAT || !req.Nat {
		e.Mode = models.DirectNAT
		if !req.Nat {
			e.Mode = ""
		}
		e.Nat = req.Nat
		e.VirtualRange = ""
	} else {
		// Switching to virtual NAT mode
		e.Mode = req.Mode
		e.Nat = req.Nat
		// Assign virtual range if switching to virtual NAT mode from a different mode,
		// or if already in virtual NAT mode but virtual range is empty
		if (oldMode != models.VirtualNAT) || (e.VirtualRange == "") {
			if err := logic.AssignVirtualRangeToEgress(&network, &e); err != nil {
				logger.Log(0, "error assigning virtual range to egress: ", err.Error())
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
				return
			}
		}
	}
	event := &models.Event{
		Action: models.Update,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: models.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   e.ID,
			Name: e.Name,
			Type: models.EgressSub,
		},
		Diff: models.Diff{
			Old: e,
		},
		NetworkID: models.NetworkID(e.Network),
		Origin:    models.Dashboard,
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
	if e.Domain != req.Domain {
		e.DomainAns = datatypes.JSONSlice[string]{}
	}
	// Update fields from request (Mode and Nat are already set correctly above)
	e.Range = egressRange
	e.Description = req.Description
	e.Name = req.Name
	e.Domain = req.Domain
	e.Status = req.Status
	e.UpdatedAt = time.Now().UTC()
	if err := logic.ValidateEgressReq(&e); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	// Build update map with all fields including zero values
	// GORM's Updates(&e) doesn't update zero values, so we use a map explicitly
	updateMap := map[string]any{
		"name":          e.Name,
		"description":   e.Description,
		"range":         e.Range,
		"domain":        e.Domain,
		"nat":           e.Nat,
		"mode":          e.Mode,
		"status":        e.Status,
		"nodes":         e.Nodes,
		"tags":          e.Tags,
		"domain_ans":    e.DomainAns,
		"virtual_range": e.VirtualRange,
		"updated_at":    e.UpdatedAt,
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
	if req.Domain != "" {
		if req.Nodes != nil {
			for nodeID := range req.Nodes {
				node, err := logic.GetNodeByID(nodeID)
				if err != nil {
					continue
				}
				host, _ := logic.GetHost(node.HostID.String())
				if host == nil {
					continue
				}
				mq.HostUpdate(&models.HostUpdate{
					Action: models.EgressUpdate,
					Host:   *host,
					EgressDomain: models.EgressDomain{
						ID:     e.ID,
						Host:   *host,
						Node:   node,
						Domain: e.Domain,
					},
					Node: node,
				})
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
		Action: models.Delete,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: models.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   e.ID,
			Name: e.Name,
			Type: models.EgressSub,
		},
		NetworkID: models.NetworkID(e.Network),
		Origin:    models.Dashboard,
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
