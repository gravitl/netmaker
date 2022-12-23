package controller

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

type hostNetworksUpdatePayload struct {
	Networks []string `json:"networks"`
}

func hostHandlers(r *mux.Router) {
	r.HandleFunc("/api/hosts", logic.SecurityCheck(false, http.HandlerFunc(getHosts))).Methods("GET")
	r.HandleFunc("/api/hosts/{hostid}", logic.SecurityCheck(true, http.HandlerFunc(updateHost))).Methods("PUT")
	r.HandleFunc("/api/hosts/{hostid}", logic.SecurityCheck(true, http.HandlerFunc(deleteHost))).Methods("DELETE")
	r.HandleFunc("/api/hosts/{hostid}/networks", logic.SecurityCheck(true, http.HandlerFunc(updateHostNetworks))).Methods("PUT")
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
	// return JSON/API formatted hosts
	apiHosts := logic.GetAllHostsAPI(currentHosts[:])
	logger.Log(2, r.Header.Get("user"), "fetched all hosts")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiHosts)
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
	var newHostData models.ApiHost
	err := json.NewDecoder(r.Body).Decode(&newHostData)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to update a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	// confirm host exists
	currHost, err := logic.GetHost(newHostData.ID)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to update a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	newHost := newHostData.ConvertAPIHostToNMHost(currHost)

	logic.UpdateHost(newHost, currHost) // update the in memory struct values
	if err = logic.UpsertHost(newHost); err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to update a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	apiHostData := newHost.ConvertNMHostToAPI()
	logger.Log(2, r.Header.Get("user"), "updated host", newHost.ID.String())
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiHostData)
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

	apiHostData := currHost.ConvertNMHostToAPI()
	logger.Log(2, r.Header.Get("user"), "removed host", currHost.Name)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiHostData)
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

	// TODO: add and remove hosts to networks (nodes)
	if err = logic.UpdateHostNetworks(currHost, servercfg.GetServer(), payload.Networks); err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to update host networks:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	logger.Log(2, r.Header.Get("user"), "updated host networks", currHost.Name)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(payload)
}
