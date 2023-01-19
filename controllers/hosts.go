package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
)

type hostNetworksUpdatePayload struct {
	Networks []string `json:"networks"`
}

func hostHandlers(r *mux.Router) {
	r.HandleFunc("/api/hosts", logic.SecurityCheck(true, http.HandlerFunc(getHosts))).Methods(http.MethodGet)
	r.HandleFunc("/api/hosts/{hostid}", logic.SecurityCheck(true, http.HandlerFunc(updateHost))).Methods(http.MethodPut)
	r.HandleFunc("/api/hosts/{hostid}", logic.SecurityCheck(true, http.HandlerFunc(deleteHost))).Methods(http.MethodDelete)
	r.HandleFunc("/api/hosts/{hostid}/networks/{network}", logic.SecurityCheck(true, http.HandlerFunc(addHostToNetwork))).Methods(http.MethodPost)
	r.HandleFunc("/api/hosts/{hostid}/networks/{network}", logic.SecurityCheck(true, http.HandlerFunc(deleteHostFromNetwork))).Methods(http.MethodDelete)
	r.HandleFunc("/api/hosts/{hostid}/relay", logic.SecurityCheck(false, http.HandlerFunc(createHostRelay))).Methods(http.MethodPost)
	r.HandleFunc("/api/hosts/{hostid}/relay", logic.SecurityCheck(false, http.HandlerFunc(deleteHostRelay))).Methods(http.MethodDelete)
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
	// check if relay information is changed
	updateRelay := false
	if newHost.IsRelay && len(newHost.RelayedHosts) > 0 {
		if len(newHost.RelayedHosts) != len(currHost.RelayedHosts) || !reflect.DeepEqual(newHost.RelayedHosts, currHost.RelayedHosts) {
			updateRelay = true
		}
	}

	logic.UpdateHost(newHost, currHost) // update the in memory struct values
	if err = logic.UpsertHost(newHost); err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to update a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if updateRelay {
		logic.UpdateHostRelay(currHost.ID.String(), currHost.RelayedHosts, newHost.RelayedHosts)
	}

	newNetworks := logic.GetHostNetworks(newHost.ID.String())
	if len(newNetworks) > 0 {
		if err = mq.ModifyClient(&mq.MqClient{
			ID:       currHost.ID.String(),
			Text:     currHost.Name,
			Networks: newNetworks,
		}); err != nil {
			logger.Log(0, r.Header.Get("user"), "failed to update host networks roles in DynSec:", err.Error())
		}
	}
	// publish host update through MQ
	if mq.HostUpdate(&models.HostUpdate{
		Action: models.UpdateHost,
		Host:   *newHost,
	}); err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to send host update: ", currHost.ID.String(), err.Error())
	}
	go func() {
		if err := mq.PublishPeerUpdate(); err != nil {
			logger.Log(0, "fail to publish peer update: ", err.Error())
		}
	}()

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
	if err = mq.HostUpdate(&models.HostUpdate{
		Action: models.DeleteHost,
		Host:   *currHost,
	}); err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to send delete host update: ", currHost.ID.String(), err.Error())
	}

	if err = mq.DeleteMqClient(currHost.ID.String()); err != nil {
		logger.Log(0, "error removing DynSec credentials for host:", currHost.Name, err.Error())
	}

	apiHostData := currHost.ConvertNMHostToAPI()
	logger.Log(2, r.Header.Get("user"), "removed host", currHost.Name)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiHostData)
}

// swagger:route POST /api/hosts/{hostid}/networks/{network} hosts addHostToNetwork
//
// Given a network, a host is added to the network.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: addHostToNetworkResponse
func addHostToNetwork(w http.ResponseWriter, r *http.Request) {

	var params = mux.Vars(r)
	hostid := params["hostid"]
	network := params["network"]
	if hostid == "" || network == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("hostid or network cannot be empty"), "badrequest"))
		return
	}
	// confirm host exists
	currHost, err := logic.GetHost(hostid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to find host:", hostid, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	newNode, err := logic.UpdateHostNetwork(currHost, network, true)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to add host to network:", hostid, network, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(1, "added new node", newNode.ID.String(), "to host", currHost.Name)

	networks := logic.GetHostNetworks(currHost.ID.String())
	if len(networks) > 0 {
		if err = mq.ModifyClient(&mq.MqClient{
			ID:       currHost.ID.String(),
			Text:     currHost.Name,
			Networks: networks,
		}); err != nil {
			logger.Log(0, r.Header.Get("user"), "failed to update host networks roles in DynSec:", hostid, err.Error())
		}
	}

	logger.Log(2, r.Header.Get("user"), fmt.Sprintf("added host %s to network %s", currHost.Name, network))
	w.WriteHeader(http.StatusOK)
}

// swagger:route DELETE /api/hosts/{hostid}/networks/{network} hosts deleteHostFromNetwork
//
// Given a network, a host is removed from the network.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: deleteHostFromNetworkResponse
func deleteHostFromNetwork(w http.ResponseWriter, r *http.Request) {

	var params = mux.Vars(r)
	hostid := params["hostid"]
	network := params["network"]
	if hostid == "" || network == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("hostid or network cannot be empty"), "badrequest"))
		return
	}
	// confirm host exists
	currHost, err := logic.GetHost(hostid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to find host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	node, err := logic.UpdateHostNetwork(currHost, network, false)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to remove host from network:", hostid, network, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(1, "deleting  node", node.ID.String(), "from host", currHost.Name)
	if err := logic.DeleteNode(node, false); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to delete node"), "internal"))
		return
	}
	// notify node change
	runUpdates(node, false)
	go func() { // notify of peer change
		if err := mq.PublishPeerUpdate(); err != nil {
			logger.Log(1, "error publishing peer update ", err.Error())
		}
	}()
	logger.Log(2, r.Header.Get("user"), fmt.Sprintf("removed host %s from network %s", currHost.Name, network))
	w.WriteHeader(http.StatusOK)
}
