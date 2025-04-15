package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
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
	e := models.Egress{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Network:     req.Network,
		Description: req.Description,
		Range:       req.Range,
		Nat:         req.Nat,
		Nodes:       make(datatypes.JSONMap),
		Tags:        make(datatypes.JSONMap),
		CreatedBy:   r.Header.Get("user"),
		CreatedAt:   time.Now().UTC(),
	}
	for _, nodeID := range req.Nodes {
		e.Nodes[nodeID] = struct{}{}
	}
	for _, tagID := range req.Tags {
		e.Tags[tagID] = struct{}{}
	}
	if !logic.ValidateEgressReq(&e) {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("invalid egress request"), "badrequest"))
		return
	}
	err = e.Create()
	if err != nil {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("error creating egress resource"+err.Error()), "internal"),
		)
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, req, "created egress resource")
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
	e := models.Egress{Network: network}
	list, err := e.ListByNetwork()
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
	e := models.Egress{ID: req.ID}
	err = e.Get()
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	e.Nodes = make(datatypes.JSONMap)
	e.Tags = make(datatypes.JSONMap)
	for _, nodeID := range req.Nodes {
		e.Nodes[nodeID] = struct{}{}
	}
	for _, tagID := range req.Tags {
		e.Tags[tagID] = struct{}{}
	}
	var updateNat bool
	if req.Nat != e.Nat {
		updateNat = true
	}
	e.Range = req.Range
	e.Description = req.Description
	e.Name = req.Name
	e.UpdatedAt = time.Now().UTC()
	if !logic.ValidateEgressReq(&e) {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("invalid egress request"), "badrequest"))
		return
	}
	err = e.Update()
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
		e.UpdateNatStatus()
	}
	logic.ReturnSuccessResponseWithJson(w, r, req, "updated egress resource")
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
	e := models.Egress{ID: id}
	err := e.Delete()
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, nil, "deleted egress resource")
}
