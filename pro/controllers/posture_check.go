package controllers

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
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/schema"
)

func PostureCheckHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/posture_check", logic.SecurityCheck(true, http.HandlerFunc(createPostureCheck))).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/posture_check", logic.SecurityCheck(true, http.HandlerFunc(listPostureChecks))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/posture_check", logic.SecurityCheck(true, http.HandlerFunc(updatePostureCheck))).Methods(http.MethodPut)
	r.HandleFunc("/api/v1/posture_check", logic.SecurityCheck(true, http.HandlerFunc(deletePostureCheck))).Methods(http.MethodDelete)
	r.HandleFunc("/api/v1/posture_check/attrs", logic.SecurityCheck(true, http.HandlerFunc(listPostureChecksAttrs))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/posture_check/violations", logic.SecurityCheck(true, http.HandlerFunc(listPostureCheckViolatedNodes))).Methods(http.MethodGet)
}

// @Summary     List Posture Checks Available Attributes
// @Router      /api/v1/posture_check/attrs [get]
// @Tags        Posture Check
// @Security    oauth
// @Produce     json
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     401 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func listPostureChecksAttrs(w http.ResponseWriter, r *http.Request) {

	logic.ReturnSuccessResponseWithJson(w, r, schema.PostureCheckAttrValues, "fetched posture checks")
}

// @Summary     Create Posture Check
// @Router      /api/v1/posture_check [post]
// @Tags        Posture Check
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       body body schema.PostureCheck true "Posture Check payload"
// @Success     200 {object} schema.PostureCheck
// @Failure     400 {object} models.ErrorResponse
// @Failure     401 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func createPostureCheck(w http.ResponseWriter, r *http.Request) {

	var req schema.PostureCheck
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logger.Log(0, "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if err := proLogic.ValidatePostureCheck(&req); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	pc := schema.PostureCheck{
		ID:          uuid.New().String(),
		Name:        req.Name,
		NetworkID:   req.NetworkID,
		Description: req.Description,
		Tags:        req.Tags,
		UserGroups:  req.UserGroups,
		Attribute:   req.Attribute,
		Values:      req.Values,
		Severity:    req.Severity,
		Status:      true,
		CreatedBy:   r.Header.Get("user"),
		CreatedAt:   time.Now().UTC(),
	}

	err = pc.Create(db.WithContext(r.Context()))
	if err != nil {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("error creating posture check "+err.Error()), logic.Internal),
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
			ID:   pc.ID,
			Name: pc.Name,
			Type: models.PostureCheckSub,
		},
		NetworkID: models.NetworkID(pc.NetworkID),
		Origin:    models.Dashboard,
	})

	go mq.PublishPeerUpdate(false)
	go proLogic.RunPostureChecks()
	logic.ReturnSuccessResponseWithJson(w, r, pc, "created posture check")
}

// @Summary     List Posture Checks
// @Router      /api/v1/posture_check [get]
// @Tags        Posture Check
// @Security    oauth
// @Produce     json
// @Param       network query string true "Network ID"
// @Param       id query string false "Posture Check ID to fetch a specific check"
// @Success     200 {array} schema.PostureCheck
// @Failure     400 {object} models.ErrorResponse
// @Failure     401 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func listPostureChecks(w http.ResponseWriter, r *http.Request) {

	network := r.URL.Query().Get("network")
	if network == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("network is required"), logic.BadReq))
		return
	}
	_, err := logic.GetNetwork(network)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("network not found"), logic.BadReq))
		return
	}
	id := r.URL.Query().Get("id")
	if id != "" {
		pc := schema.PostureCheck{ID: id}
		err := pc.Get(db.WithContext(r.Context()))
		if err != nil {
			logic.ReturnErrorResponse(
				w,
				r,
				logic.FormatError(errors.New("error listing posture checks "+err.Error()), "internal"),
			)
			return
		}
		logic.ReturnSuccessResponseWithJson(w, r, pc, "fetched posture check")
		return
	}
	pc := schema.PostureCheck{NetworkID: models.NetworkID(network)}
	list, err := pc.ListByNetwork(db.WithContext(r.Context()))
	if err != nil {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("error listing posture checks "+err.Error()), "internal"),
		)
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, list, "fetched posture checks")
}

// @Summary     Update Posture Check
// @Router      /api/v1/posture_check [put]
// @Tags        Posture Check
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       body body schema.PostureCheck true "Posture Check payload"
// @Success     200 {object} schema.PostureCheck
// @Failure     400 {object} models.ErrorResponse
// @Failure     401 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func updatePostureCheck(w http.ResponseWriter, r *http.Request) {

	var updatePc schema.PostureCheck
	err := json.NewDecoder(r.Body).Decode(&updatePc)
	if err != nil {
		logger.Log(0, "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	if err := proLogic.ValidatePostureCheck(&updatePc); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	pc := schema.PostureCheck{ID: updatePc.ID}
	err = pc.Get(db.WithContext(r.Context()))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	var updateStatus bool
	if updatePc.Status != pc.Status {
		updateStatus = true
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
			ID:   pc.ID,
			Name: updatePc.Name,
			Type: models.PostureCheckSub,
		},
		Diff: models.Diff{
			Old: pc,
			New: updatePc,
		},
		NetworkID: models.NetworkID(pc.NetworkID),
		Origin:    models.Dashboard,
	}
	pc.Tags = updatePc.Tags
	pc.UserGroups = updatePc.UserGroups
	pc.Attribute = updatePc.Attribute
	pc.Values = updatePc.Values
	pc.Description = updatePc.Description
	pc.Name = updatePc.Name
	pc.Severity = updatePc.Severity
	pc.Status = updatePc.Status
	pc.UpdatedAt = time.Now().UTC()

	err = pc.Update(db.WithContext(context.TODO()))
	if err != nil {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("error updating posture check "+err.Error()), "internal"),
		)
		return
	}
	if updateStatus {
		pc.UpdateStatus(db.WithContext(context.TODO()))
	}
	logic.LogEvent(event)
	go mq.PublishPeerUpdate(false)
	go proLogic.RunPostureChecks()
	logic.ReturnSuccessResponseWithJson(w, r, pc, "updated posture check")
}

// @Summary     Delete Posture Check
// @Router      /api/v1/posture_check [delete]
// @Tags        Posture Check
// @Security    oauth
// @Produce     json
// @Param       id query string true "Posture Check ID"
// @Success     200 {object} schema.PostureCheck
// @Failure     400 {object} models.ErrorResponse
// @Failure     401 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func deletePostureCheck(w http.ResponseWriter, r *http.Request) {

	id := r.URL.Query().Get("id")
	if id == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("id is required"), "badrequest"))
		return
	}
	pc := schema.PostureCheck{ID: id}
	err := pc.Get(db.WithContext(r.Context()))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}
	err = pc.Delete(db.WithContext(r.Context()))
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
			ID:   pc.ID,
			Name: pc.Name,
			Type: models.PostureCheckSub,
		},
		NetworkID: models.NetworkID(pc.NetworkID),
		Origin:    models.Dashboard,
		Diff: models.Diff{
			Old: pc,
			New: nil,
		},
	})

	go mq.PublishPeerUpdate(false)
	logic.ReturnSuccessResponseWithJson(w, r, pc, "deleted posture check")
}

// @Summary     List Posture Check violated Nodes
// @Router      /api/v1/posture_check/violations [get]
// @Tags        Posture Check
// @Security    oauth
// @Produce     json
// @Param       network query string true "Network ID"
// @Param       users query string false "If 'true', list violated users instead of nodes"
// @Success     200 {array} models.ApiNode
// @Failure     400 {object} models.ErrorResponse
// @Failure     401 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func listPostureCheckViolatedNodes(w http.ResponseWriter, r *http.Request) {

	network := r.URL.Query().Get("network")
	if network == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("network is required"), logic.BadReq))
		return
	}
	listViolatedusers := r.URL.Query().Get("users") == "true"
	violatedNodes := []models.Node{}
	if listViolatedusers {
		extclients, err := logic.GetNetworkExtClients(network)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
			return
		}
		for _, extclient := range extclients {
			if extclient.DeviceID != "" {
				if len(extclient.PostureChecksViolations) > 0 {
					violatedNodes = append(violatedNodes, extclient.ConvertToStaticNode())
				}
			}
		}
	} else {
		nodes, err := logic.GetNetworkNodes(network)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
			return
		}

		for _, node := range nodes {
			if len(node.PostureChecksViolations) > 0 {
				violatedNodes = append(violatedNodes, node)
			}
		}
	}
	apiNodes := logic.GetAllNodesAPI(violatedNodes)
	logic.SortApiNodes(apiNodes[:])
	logic.ReturnSuccessResponseWithJson(w, r, apiNodes, "fetched posture checks violated nodes")
}
