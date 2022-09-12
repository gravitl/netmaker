package controller

import (
	"encoding/json"
	"errors"
	"github.com/gravitl/netmaker/logger"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logic/pro"
	"github.com/gravitl/netmaker/models/promodels"
)

func userGroupsHandlers(r *mux.Router) {
	r.HandleFunc("/api/usergroups", securityCheck(true, http.HandlerFunc(getUserGroups))).Methods("GET")
	r.HandleFunc("/api/usergroups/{usergroup}", securityCheck(true, http.HandlerFunc(createUserGroup))).Methods("POST")
	r.HandleFunc("/api/usergroups/{usergroup}", securityCheck(true, http.HandlerFunc(deleteUserGroup))).Methods("DELETE")
}

func getUserGroups(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	logger.Log(1, r.Header.Get("user"), "requested fetching user groups")

	userGroups, err := pro.GetUserGroups()
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
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
		returnErrorResponse(w, r, formatError(errors.New("no group name provided"), "badrequest"))
		return
	}

	err := pro.InsertUserGroup(promodels.UserGroupName(newGroup))
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}

	w.WriteHeader(http.StatusOK)
}

func deleteUserGroup(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	groupToDelete := params["usergroup"]
	logger.Log(1, r.Header.Get("user"), "requested deleting user group", groupToDelete)

	if groupToDelete == "" {
		returnErrorResponse(w, r, formatError(errors.New("no group name provided"), "badrequest"))
		return
	}

	if err := pro.DeleteUserGroup(promodels.UserGroupName(groupToDelete)); err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	w.WriteHeader(http.StatusOK)
}
