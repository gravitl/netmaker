package ee_controllers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logic/pro"
	"github.com/gravitl/netmaker/models/promodels"
)

func UserGroupsHandlers(r *mux.Router) {
	r.HandleFunc("/api/usergroups", logic.SecurityCheck(true, http.HandlerFunc(getUserGroups))).Methods("GET")
	r.HandleFunc("/api/usergroups/{usergroup}", logic.SecurityCheck(true, http.HandlerFunc(createUserGroup))).Methods("POST")
	r.HandleFunc("/api/usergroups/{usergroup}", logic.SecurityCheck(true, http.HandlerFunc(deleteUserGroup))).Methods("DELETE")
}

func getUserGroups(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	logger.Log(1, r.Header.Get("user"), "requested fetching user groups")

	userGroups, err := pro.GetUserGroups()
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	// Returns all the groups in JSON format
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(userGroups)
}

func createUserGroup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	newGroup := params["usergroup"]

	logger.Log(1, r.Header.Get("user"), "requested creating user group", newGroup)

	if newGroup == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("no group name provided"), "badrequest"))
		return
	}

	err := pro.InsertUserGroup(promodels.UserGroupName(newGroup))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	w.WriteHeader(http.StatusOK)
}

func deleteUserGroup(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	groupToDelete := params["usergroup"]
	logger.Log(1, r.Header.Get("user"), "requested deleting user group", groupToDelete)

	if groupToDelete == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("no group name provided"), "badrequest"))
		return
	}

	if err := pro.DeleteUserGroup(promodels.UserGroupName(groupToDelete)); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	w.WriteHeader(http.StatusOK)
}
