package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
)

func aclHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/acls", logic.SecurityCheck(true, http.HandlerFunc(getAcls))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/v1/acls/policy_types", logic.SecurityCheck(true, http.HandlerFunc(aclPolicyTypes))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/v1/acls", logic.SecurityCheck(true, http.HandlerFunc(createAcl))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/v1/acls", logic.SecurityCheck(true, http.HandlerFunc(updateAcl))).
		Methods(http.MethodPut)
	r.HandleFunc("/api/v1/acls", logic.SecurityCheck(true, http.HandlerFunc(deleteAcl))).
		Methods(http.MethodDelete)
	r.HandleFunc("/api/v1/acls/debug", logic.SecurityCheck(true, http.HandlerFunc(aclDebug))).
		Methods(http.MethodGet)
}

// @Summary     List Acl Policy types
// @Router      /api/v1/acls/policy_types [get]
// @Tags        ACL
// @Accept      json
// @Success     200 {array} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func aclPolicyTypes(w http.ResponseWriter, r *http.Request) {
	resp := models.AclPolicyTypes{
		RuleTypes: []models.AclPolicyType{
			models.DevicePolicy,
			models.UserPolicy,
		},
		SrcGroupTypes: []models.AclGroupType{
			models.UserAclID,
			models.UserGroupAclID,
			models.DeviceAclID,
		},
		DstGroupTypes: []models.AclGroupType{
			models.DeviceAclID,
			// models.NetmakerIPAclID,
			// models.NetmakerSubNetRangeAClID,
		},
	}
	logic.ReturnSuccessResponseWithJson(w, r, resp, "fetched acls types")
}

func aclDebug(w http.ResponseWriter, r *http.Request) {
	nodeID, _ := url.QueryUnescape(r.URL.Query().Get("node"))
	peerID, _ := url.QueryUnescape(r.URL.Query().Get("peer"))
	node, err := logic.GetNodeByID(nodeID)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	peer, err := logic.GetNodeByID(peerID)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	allowed := logic.IsNodeAllowedToCommunicate(node, peer, true)
	logic.ReturnSuccessResponseWithJson(w, r, allowed, "fetched all acls in the network ")
}

// @Summary     List Acls in a network
// @Router      /api/v1/acls [get]
// @Tags        ACL
// @Accept      json
// @Success     200 {array} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func getAcls(w http.ResponseWriter, r *http.Request) {
	netID := r.URL.Query().Get("network")
	if netID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("network id param is missing"), "badrequest"))
		return
	}
	// check if network exists
	_, err := logic.GetNetwork(netID)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	acls, err := logic.ListAcls(models.NetworkID(netID))
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to get all network acl entries: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.SortAclEntrys(acls[:])
	logic.ReturnSuccessResponseWithJson(w, r, acls, "fetched all acls in the network "+netID)
}

// @Summary     Create Acl
// @Router      /api/v1/acls [post]
// @Tags        ACL
// @Accept      json
// @Success     200 {array} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func createAcl(w http.ResponseWriter, r *http.Request) {
	var req models.Acl
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logger.Log(0, "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	user, err := logic.GetUser(r.Header.Get("user"))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	err = logic.ValidateCreateAclReq(req)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	acl := req
	acl.ID = uuid.New().String()
	acl.CreatedBy = user.UserName
	acl.CreatedAt = time.Now().UTC()
	acl.Default = false
	if acl.RuleType == models.DevicePolicy {
		acl.AllowedDirection = models.TrafficDirectionBi
	} else {
		acl.AllowedDirection = models.TrafficDirectionUni
	}
	// validate create acl policy
	if !logic.IsAclPolicyValid(acl) {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("invalid policy"), "badrequest"))
		return
	}
	err = logic.InsertAcl(acl)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	acl, err = logic.GetAcl(acl.ID)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	go mq.PublishPeerUpdate(false)
	logic.ReturnSuccessResponseWithJson(w, r, acl, "created acl successfully")
}

// @Summary     Update Acl
// @Router      /api/v1/acls [put]
// @Tags        ACL
// @Accept      json
// @Success     200 {array} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func updateAcl(w http.ResponseWriter, r *http.Request) {
	var updateAcl models.UpdateAclRequest
	err := json.NewDecoder(r.Body).Decode(&updateAcl)
	if err != nil {
		logger.Log(0, "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	acl, err := logic.GetAcl(updateAcl.ID)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if !logic.IsAclPolicyValid(updateAcl.Acl) {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("invalid policy"), "badrequest"))
		return
	}
	if updateAcl.Acl.NetworkID != acl.NetworkID {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("invalid policy, network id mismatch"), "badrequest"))
		return
	}
	if !acl.Default && updateAcl.NewName != "" {
		//check if policy exists with same name
		updateAcl.Acl.Name = updateAcl.NewName
	}
	err = logic.UpdateAcl(updateAcl.Acl, acl)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	go mq.PublishPeerUpdate(false)
	logic.ReturnSuccessResponse(w, r, "updated acl "+acl.Name)
}

// @Summary     Delete Acl
// @Router      /api/v1/acls [delete]
// @Tags        ACL
// @Accept      json
// @Success     200 {array} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func deleteAcl(w http.ResponseWriter, r *http.Request) {
	aclID, _ := url.QueryUnescape(r.URL.Query().Get("acl_id"))
	if aclID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("acl id is required"), "badrequest"))
		return
	}
	acl, err := logic.GetAcl(aclID)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if acl.Default {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("cannot delete default policy"), "badrequest"))
		return
	}
	err = logic.DeleteAcl(acl)
	if err != nil {
		logic.ReturnErrorResponse(w, r,
			logic.FormatError(errors.New("cannot delete default policy"), "internal"))
		return
	}
	go mq.PublishPeerUpdate(false)
	logic.ReturnSuccessResponse(w, r, "deleted acl "+acl.Name)
}
