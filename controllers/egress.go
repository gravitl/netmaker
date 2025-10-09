package controller

import (
	"context"
	"encoding/json"
	"errors"
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
// @Tags        Auth
// @Accept      json
// @Param       body body models.Egress
// @Success     200 {object} models.SuccessResponse
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

	e := schema.Egress{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Network:     req.Network,
		Description: req.Description,
		Range:       egressRange,
		Domain:      req.Domain,
		DomainAns:   []string{},
		Nat:         req.Nat,
		Nodes:       make(datatypes.JSONMap),
		Tags:        make(datatypes.JSONMap),
		Status:      true,
		CreatedBy:   r.Header.Get("user"),
		CreatedAt:   time.Now().UTC(),
	}
	for nodeID, metric := range req.Nodes {
		e.Nodes[nodeID] = metric
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

// @Summary     List Egress Resource
// @Router      /api/v1/egress [get]
// @Tags        Auth
// @Accept      json
// @Param       query network string
// @Success     200 {object} models.SuccessResponse
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
// @Tags        Auth
// @Accept      json
// @Param       body body models.Egress
// @Success     200 {object} models.SuccessResponse
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
	var updateNat bool
	var updateStatus bool
	var resetDomain bool
	var resetRange bool
	if req.Nat != e.Nat {
		updateNat = true
	}
	if req.Status != e.Status {
		updateStatus = true
	}
	if req.Domain == "" {
		resetDomain = true
	}
	if req.Range == "" || egressRange == "" {
		resetRange = true
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
	for nodeID, metric := range req.Nodes {
		e.Nodes[nodeID] = metric
	}
	if e.Domain != req.Domain {
		e.DomainAns = datatypes.JSONSlice[string]{}
	}
	e.Range = egressRange
	e.Description = req.Description
	e.Name = req.Name
	e.Nat = req.Nat
	e.Domain = req.Domain
	e.Status = req.Status
	e.UpdatedAt = time.Now().UTC()
	if err := logic.ValidateEgressReq(&e); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	err = e.Update(db.WithContext(context.TODO()))
	if err != nil {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("error creating egress resource"+err.Error()), "internal"),
		)
		return
	}
	if updateNat {
		e.Nat = req.Nat
		e.UpdateNatStatus(db.WithContext(context.TODO()))
	}
	if updateStatus {
		e.Status = req.Status
		e.UpdateEgressStatus(db.WithContext(context.TODO()))
	}
	if resetDomain {
		_ = e.ResetDomain(db.WithContext(context.TODO()))
	}
	if resetRange {
		_ = e.ResetRange(db.WithContext(context.TODO()))
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
// @Tags        Auth
// @Accept      json
// @Param       body body models.Egress
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
