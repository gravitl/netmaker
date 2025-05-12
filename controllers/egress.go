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
		egressRange, err = logic.NormalizeCIDR(req.Range)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
	} else {
		egressRange = "*"
	}

	e := schema.Egress{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Network:     req.Network,
		Description: req.Description,
		Range:       egressRange,
		Nat:         req.Nat,
		IsInetGw:    req.IsInetGw,
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
	// for nodeID := range e.Nodes {
	// 	node, err := logic.GetNodeByID(nodeID)
	// 	if err != nil {
	// 		logic.AddEgressInfoToNode(&node, e)
	// 		logic.UpsertNode(&node)
	// 	}

	// }
	go mq.PublishPeerUpdate(false)
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
		egressRange, err = logic.NormalizeCIDR(req.Range)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
	} else {
		egressRange = "*"
	}

	e := schema.Egress{ID: req.ID}
	err = e.Get(db.WithContext(r.Context()))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	var updateNat bool
	var updateInetGw bool
	var updateStatus bool
	if req.Nat != e.Nat {
		updateNat = true
	}
	if req.IsInetGw != e.IsInetGw {
		updateInetGw = true
	}
	if req.Status != e.Status {
		updateStatus = true
	}
	e.Nodes = make(datatypes.JSONMap)
	e.Tags = make(datatypes.JSONMap)
	for nodeID, metric := range req.Nodes {
		e.Nodes[nodeID] = metric
	}
	e.Range = egressRange
	e.Description = req.Description
	e.Name = req.Name
	e.Nat = req.Nat
	e.Status = req.Status
	e.IsInetGw = req.IsInetGw
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
	if updateInetGw {
		e.IsInetGw = req.IsInetGw
		e.UpdateINetGwStatus(db.WithContext(context.TODO()))
	}
	if updateStatus {
		e.Status = req.Status
		e.UpdateEgressStatus(db.WithContext(context.TODO()))
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
	err := e.Delete(db.WithContext(r.Context()))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.CleanUpEgressPolicies()
	go mq.PublishPeerUpdate(false)
	logic.ReturnSuccessResponseWithJson(w, r, nil, "deleted egress resource")
}
