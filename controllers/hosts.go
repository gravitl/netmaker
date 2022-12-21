package controller

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

type hostNetworksUpdatePayload struct {
	Networks []string `json:"networks"`
}

func hostHandlers(r *mux.Router) {
	r.HandleFunc("/api/hosts", logic.SecurityCheck(false, http.HandlerFunc(getHosts))).Methods("GET")
	r.HandleFunc("/api/hosts", logic.SecurityCheck(true, http.HandlerFunc(updateHost))).Methods("PUT")
	r.HandleFunc("/api/hosts/{hostid}", logic.SecurityCheck(true, http.HandlerFunc(deleteHost))).Methods("DELETE")
	r.HandleFunc("/api/hosts/{hostid}", logic.SecurityCheck(true, http.HandlerFunc(updateHostNetworks))).Methods("PUT")
}

// swagger:route GET /api/hosts hosts getHosts
//
// Lists all hosts.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: getHostsSliceResponse
func getHosts(w http.ResponseWriter, r *http.Request) {
	currentHosts, err := logic.GetAllHosts()
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to fetch hosts: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(2, r.Header.Get("user"), "fetched all hosts")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(currentHosts)
}

// swagger:route PUT /api/hosts/{hostid} hosts updateHost
//
// Updates a Netclient host on Netmaker server.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: updateHostResponse
func updateHost(w http.ResponseWriter, r *http.Request) {
	var newHostData models.Host
	err := json.NewDecoder(r.Body).Decode(&newHostData)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to update a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	// confirm host exists
	currHost, err := logic.GetHost(newHostData.ID.String())
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to update a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	logic.UpdateHost(&newHostData, currHost) // update the in memory struct values
	if err = logic.UpsertHost(&newHostData); err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to update a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	logger.Log(2, r.Header.Get("user"), "updated host", newHostData.ID.String())
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(newHostData)
}

// swagger:route DELETE /api/hosts/{hostid} hosts deleteHost
//
// Deletes a Netclient host from Netmaker server.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: deleteHostResponse
func deleteHost(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	hostid := params["hostid"]
	// confirm host exists
	currHost, err := logic.GetHost(hostid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to delete a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	if err = logic.RemoveHost(currHost); err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to delete a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	logger.Log(2, r.Header.Get("user"), "removed host", currHost.Name)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(currHost)
}

// swagger:route PUT /api/hosts hosts updateHostNetworks
//
// Given a list of networks, a host is updated accordingly.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: updateHostNetworks
func updateHostNetworks(w http.ResponseWriter, r *http.Request) {
	var payload hostNetworksUpdatePayload
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to update host networks:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	// confirm host exists
	var params = mux.Vars(r)
	hostid := params["hostid"]
	currHost, err := logic.GetHost(hostid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to find host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	logger.Log(2, r.Header.Get("user"), "updated host", currHost.Name)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(payload)
}
