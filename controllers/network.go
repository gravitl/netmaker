package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"golang.org/x/exp/slog"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/acls"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/servercfg"
)

func networkHandlers(r *mux.Router) {
	r.HandleFunc("/api/networks", logic.SecurityCheck(true, http.HandlerFunc(getNetworks))).Methods(http.MethodGet)
	r.HandleFunc("/api/networks", logic.SecurityCheck(true, checkFreeTierLimits(limitChoiceNetworks, http.HandlerFunc(createNetwork)))).Methods(http.MethodPost)
	r.HandleFunc("/api/networks/{networkname}", logic.SecurityCheck(true, http.HandlerFunc(getNetwork))).Methods(http.MethodGet)
	r.HandleFunc("/api/networks/{networkname}", logic.SecurityCheck(true, http.HandlerFunc(deleteNetwork))).Methods(http.MethodDelete)
	r.HandleFunc("/api/networks/{networkname}", logic.SecurityCheck(true, http.HandlerFunc(updateNetwork))).Methods(http.MethodPut)
	// ACLs
	r.HandleFunc("/api/networks/{networkname}/acls", logic.SecurityCheck(true, http.HandlerFunc(updateNetworkACL))).Methods(http.MethodPut)
	r.HandleFunc("/api/networks/{networkname}/acls", logic.SecurityCheck(true, http.HandlerFunc(getNetworkACL))).Methods(http.MethodGet)
}

// swagger:route GET /api/networks networks getNetworks
//
// Lists all networks.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: getNetworksSliceResponse
func getNetworks(w http.ResponseWriter, r *http.Request) {

	var err error

	allnetworks, err := logic.GetNetworks()
	if err != nil && !database.IsEmptyRecord(err) {
		logger.Log(0, r.Header.Get("user"), "failed to fetch networks: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	logger.Log(2, r.Header.Get("user"), "fetched networks.")
	logic.SortNetworks(allnetworks[:])
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(allnetworks)
}

// swagger:route GET /api/networks/{networkname} networks getNetwork
//
// Get a network.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: networkBodyResponse
func getNetwork(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	netname := params["networkname"]
	network, err := logic.GetNetwork(netname)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), fmt.Sprintf("failed to fetch network [%s] info: %v",
			netname, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	logger.Log(2, r.Header.Get("user"), "fetched network", netname)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(network)
}

// swagger:route PUT /api/networks/{networkname}/acls networks updateNetworkACL
//
// Update a network ACL (Access Control List).
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: aclContainerResponse
func updateNetworkACL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	netname := params["networkname"]
	var networkACLChange acls.ACLContainer
	networkACLChange, err := networkACLChange.Get(acls.ContainerID(netname))
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to fetch ACLs for network [%s]: %v", netname, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	err = json.NewDecoder(r.Body).Decode(&networkACLChange)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	newNetACL, err := networkACLChange.Save(acls.ContainerID(netname))
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to update ACLs for network [%s]: %v", netname, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "updated ACLs for network", netname)

	// send peer updates
	if servercfg.IsMessageQueueBackend() {
		if err = mq.PublishPeerUpdate(); err != nil {
			logger.Log(0, "failed to publish peer update after ACL update on", netname)
		}
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(newNetACL)
}

// swagger:route GET /api/networks/{networkname}/acls networks getNetworkACL
//
// Get a network ACL (Access Control List).
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: aclContainerResponse
func getNetworkACL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	netname := params["networkname"]
	var networkACL acls.ACLContainer
	networkACL, err := networkACL.Get(acls.ContainerID(netname))
	if err != nil {
		if database.IsEmptyRecord(err) {
			networkACL = acls.ACLContainer{}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(networkACL)
			return
		}
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to fetch ACLs for network [%s]: %v", netname, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(2, r.Header.Get("user"), "fetched acl for network", netname)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(networkACL)
}

// swagger:route DELETE /api/networks/{networkname} networks deleteNetwork
//
// Delete a network.  Will not delete if there are any nodes that belong to the network.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: stringJSONResponse
func deleteNetwork(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	network := params["networkname"]
	err := logic.DeleteNetwork(network)
	if err != nil {
		errtype := "badrequest"
		if strings.Contains(err.Error(), "Node check failed") {
			errtype = "forbidden"
		}
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to delete network [%s]: %v", network, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, errtype))
		return
	}

	logger.Log(1, r.Header.Get("user"), "deleted network", network)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode("success")
}

// swagger:route POST /api/networks networks createNetwork
//
// Create a network.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: networkBodyResponse
func createNetwork(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var network models.Network

	// we decode our body request params
	err := json.NewDecoder(r.Body).Decode(&network)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	if len(network.NetID) > 32 {
		err := errors.New("network name shouldn't exceed 32 characters")
		logger.Log(0, r.Header.Get("user"), "failed to create network: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	if network.AddressRange == "" && network.AddressRange6 == "" {
		err := errors.New("IPv4 or IPv6 CIDR required")
		logger.Log(0, r.Header.Get("user"), "failed to create network: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	network, err = logic.CreateNetwork(network)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to create network: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	defaultHosts := logic.GetDefaultHosts()
	for i := range defaultHosts {
		currHost := &defaultHosts[i]
		newNode, err := logic.UpdateHostNetwork(currHost, network.NetID, true)
		if err != nil {
			logger.Log(0, r.Header.Get("user"), "failed to add host to network:", currHost.ID.String(), network.NetID, err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
		logger.Log(1, "added new node", newNode.ID.String(), "to host", currHost.Name)
		if err = mq.HostUpdate(&models.HostUpdate{
			Action: models.JoinHostToNetwork,
			Host:   *currHost,
			Node:   *newNode,
		}); err != nil {
			logger.Log(0, r.Header.Get("user"), "failed to add host to network:", currHost.ID.String(), network.NetID, err.Error())
		}
	}

	logger.Log(1, r.Header.Get("user"), "created network", network.NetID)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(network)
}

// swagger:route PUT /api/networks networks updateNetwork
//
// Update pro settings for a network.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: networkBodyResponse
func updateNetwork(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var payload models.Network

	// we decode our body request params
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		slog.Info("error decoding request body", "user", r.Header.Get("user"), "err", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	netOld1, err := logic.GetNetwork(payload.NetID)
	if err != nil {
		slog.Info("error fetching network", "user", r.Header.Get("user"), "err", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	// partial update
	netOld2 := netOld1
	_, _, _, err = logic.UpdateNetwork(&netOld1, &netOld2)
	if err != nil {
		slog.Info("failed to update network", "user", r.Header.Get("user"), "err", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	slog.Info("updated network", "network", payload.NetID, "user", r.Header.Get("user"))
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(payload)
}
