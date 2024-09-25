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
)

func aclHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/acls", logic.SecurityCheck(true, http.HandlerFunc(getAcls))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/v1/acls", logic.SecurityCheck(true, http.HandlerFunc(createAcl))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/v1/acls", logic.SecurityCheck(true, http.HandlerFunc(updateAcl))).
		Methods(http.MethodPut)
	r.HandleFunc("/api/v1/acls", logic.SecurityCheck(true, http.HandlerFunc(deleteAcl))).
		Methods(http.MethodDelete)

}

// @Summary     List Acls in a network
// @Router      /api/v1/acls [get]
// @Tags        ACL
// @Accept      json
// @Success     200 {array} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func getAcls(w http.ResponseWriter, r *http.Request) {
	netID, _ := url.QueryUnescape(r.URL.Query().Get("network"))
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
	// check if acl network exists
	_, err = logic.GetNetwork(req.NetworkID.String())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("failed to get network details for "+req.NetworkID.String()), "badrequest"))
		return
	}
	// check if acl exists
	acl := req
	acl.ID = uuid.New()
	acl.CreatedBy = user.UserName
	acl.CreatedAt = time.Now().UTC()
	// validate create acl policy
	if !logic.IsAclPolicyValid(acl) {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("invalid policy"), "badrequest"))
		return
	}
	err = logic.InsertAcl(acl)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	logic.ReturnSuccessResponseWithJson(w, r, req, "created acl successfully")
}

// @Summary     Update Acl
// @Router      /api/v1/acls [put]
// @Tags        ACL
// @Accept      json
// @Success     200 {array} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func updateAcl(w http.ResponseWriter, r *http.Request) {
	var updateAcl models.Acl
	err := json.NewDecoder(r.Body).Decode(&updateAcl)
	if err != nil {
		logger.Log(0, "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	acl, err := logic.GetAcl(updateAcl.ID.String())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if !logic.IsAclPolicyValid(updateAcl) {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("invalid policy"), "badrequest"))
		return
	}
	err = logic.UpdateAcl(updateAcl, acl)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logic.ReturnSuccessResponse(w, r, "updated acl "+updateAcl.Name)
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
	err = logic.DeleteAcl(acl)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.ReturnSuccessResponse(w, r, "deleted acl "+acl.Name)
}
