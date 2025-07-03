package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gravitl/netmaker/logic/acls"
	"github.com/gravitl/netmaker/schema"
	"net"
	"net/http"
	"slices"
	"strings"

	"github.com/gorilla/mux"
	"golang.org/x/exp/slog"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/servercfg"
)

func networkHandlers(r *mux.Router) {
	r.HandleFunc("/api/networks", logic.SecurityCheck(true, http.HandlerFunc(getNetworks))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/v1/networks/stats", logic.SecurityCheck(true, http.HandlerFunc(getNetworksStats))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/networks", logic.SecurityCheck(true, checkFreeTierLimits(limitChoiceNetworks, http.HandlerFunc(createNetwork)))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/networks/{networkname}", logic.SecurityCheck(true, http.HandlerFunc(getNetwork))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/networks/{networkname}", logic.SecurityCheck(true, http.HandlerFunc(deleteNetwork))).
		Methods(http.MethodDelete)
	r.HandleFunc("/api/networks/{networkname}", logic.SecurityCheck(true, http.HandlerFunc(updateNetwork))).
		Methods(http.MethodPut)
	// ACLs
	r.HandleFunc("/api/networks/{networkname}/acls", logic.SecurityCheck(true, http.HandlerFunc(updateNetworkACL))).
		Methods(http.MethodPut)
	r.HandleFunc("/api/networks/{networkname}/acls/v2", logic.SecurityCheck(true, http.HandlerFunc(updateNetworkACLv2))).
		Methods(http.MethodPut)
	r.HandleFunc("/api/networks/{networkname}/acls", logic.SecurityCheck(true, http.HandlerFunc(getNetworkACL))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/networks/{networkname}/egress_routes", logic.SecurityCheck(true, http.HandlerFunc(getNetworkEgressRoutes)))
}

// @Summary     Lists all networks
// @Router      /api/networks [get]
// @Tags        Networks
// @Security    oauth
// @Produce     json
// @Success     200 {object} models.Network
// @Failure     500 {object} models.ErrorResponse
func getNetworks(w http.ResponseWriter, r *http.Request) {

	var err error

	allnetworks, err := logic.GetNetworks()
	if err != nil && !database.IsEmptyRecord(err) {
		slog.Error("failed to fetch networks", "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if r.Header.Get("ismaster") != "yes" {
		username := r.Header.Get("user")
		user, err := logic.GetUser(username)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
		allnetworks = logic.FilterNetworksByRole(allnetworks, *user)
	}

	logger.Log(2, r.Header.Get("user"), "fetched networks.")

	slices.SortFunc(allnetworks, func(a, b models.Network) int {
		return strings.Compare(a.NetID, b.NetID)
	})

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(allnetworks)
}

// @Summary     Lists all networks with stats
// @Router      /api/v1/networks/stats [get]
// @Tags        Networks
// @Security    oauth
// @Produce     json
// @Success     200 {object} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func getNetworksStats(w http.ResponseWriter, r *http.Request) {

	var err error
	allnetworks, err := logic.GetNetworks()
	if err != nil && !database.IsEmptyRecord(err) {
		slog.Error("failed to fetch networks", "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if r.Header.Get("ismaster") != "yes" {
		username := r.Header.Get("user")
		user, err := logic.GetUser(username)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
		allnetworks = logic.FilterNetworksByRole(allnetworks, *user)
	}

	slices.SortFunc(allnetworks, func(a, b models.Network) int {
		return strings.Compare(a.NetID, b.NetID)
	})

	var netstats []models.NetworkStatResp

	for _, network := range allnetworks {
		_network := &schema.Network{
			ID: network.NetID,
		}
		numNodes, _ := _network.CountNodes(r.Context())
		netstats = append(netstats, models.NetworkStatResp{
			Network: network,
			Hosts:   numNodes,
		})
	}

	logger.Log(2, r.Header.Get("user"), "fetched networks.")
	logic.ReturnSuccessResponseWithJson(w, r, netstats, "fetched networks with stats")
}

// @Summary     Get a network
// @Router      /api/networks/{networkname} [get]
// @Tags        Networks
// @Security    oauth
// @Param       networkname path string true "Network name"
// @Produce     json
// @Success     200 {object} models.Network
// @Failure     500 {object} models.ErrorResponse
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

// @Summary     Update a network ACL (Access Control List)
// @Router      /api/networks/{networkname}/acls [put]
// @Tags        Networks
// @Security    oauth
// @Param       networkname path string true "Network name"
// @Param       body body acls.ACLContainer true "ACL container"
// @Produce     json
// @Success     200 {object} acls.ACLContainer
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func updateNetworkACL(w http.ResponseWriter, r *http.Request) {
	networkID := mux.Vars(r)["networkname"]

	var networkACLUpdateRequest acls.ACLContainer
	err := json.NewDecoder(r.Body).Decode(&networkACLUpdateRequest)
	if err != nil {
		logger.Log(0, fmt.Sprintf("failed to decode network (%s) acl update request: %s", networkID, err.Error()))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	var networkACL acls.ACLContainer
	networkACL, err = networkACL.Get(acls.ContainerID(networkID))
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to fetch ACLs for network [%s]: %v", networkID, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	newNetACL, err := networkACL.Save(acls.ContainerID(networkID))
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to update ACLs for network [%s]: %v", networkID, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "updated ACLs for network", networkID)

	// send peer updates
	go func() {
		if err = mq.PublishPeerUpdate(false); err != nil {
			logger.Log(0, "failed to publish peer update after ACL update on network:", networkID)
		}
	}()

	logic.ReturnSuccessJsonResponse(w, r, newNetACL)
}

// @Summary     Update a network ACL (Access Control List)
// @Router      /api/networks/{networkname}/acls/v2 [put]
// @Tags        Networks
// @Security    oauth
// @Param       networkname path string true "Network name"
// @Param       body body acls.ACLContainer true "ACL container"
// @Produce     json
// @Success     200 {object} acls.ACLContainer
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func updateNetworkACLv2(w http.ResponseWriter, r *http.Request) {
	networkID := mux.Vars(r)["networkname"]

	var networkACLUpdateRequest acls.ACLContainer
	err := json.NewDecoder(r.Body).Decode(&networkACLUpdateRequest)
	if err != nil {
		logger.Log(0, fmt.Sprintf("failed to decode network (%s) acl update request: %s", networkID, err.Error()))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	var networkACL acls.ACLContainer
	networkACL, err = networkACL.Get(acls.ContainerID(networkID))
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to fetch ACLs for network [%s]: %v", networkID, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	_network := &schema.Network{
		ID: networkID,
	}
	_networkNodes, err := _network.GetNodes(r.Context())
	if err != nil {
		logger.Log(0, fmt.Sprintf("failed to get network (%s) nodes: %s", networkID, err.Error()))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	nodesMap := make(map[string]schema.Node)
	for _, _node := range _networkNodes {
		nodesMap[_node.ID] = _node
	}

	extClients, err := logic.GetNetworkExtClients(networkID)
	if err != nil {
		logger.Log(0, fmt.Sprintf("failed to get network (%s) extClients: %s", networkID, err.Error()))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	extClientsMap := make(map[string]models.ExtClient)
	for _, extClient := range extClients {
		extClientsMap[extClient.ClientID] = extClient
	}

	// keep track of ingress gateways to disconnect from their clients
	// this is required because PublishPeerUpdate only somehow does not stop communication
	// between blocked clients and their ingress
	assocClientsToDisconnectPerHost := make(map[string][]models.ExtClient)

	// update client acls and then, remove client acls from req data to pass to existing functions
	for id, acl := range networkACLUpdateRequest {
		// for node acls
		if _, ok := nodesMap[string(id)]; ok {
			nodeID := string(id)
			// check acl update, then remove client entries
			for id2 := range acl {
				if _, ok := nodesMap[string(id2)]; !ok {
					// update client acl
					clientID := string(id2)
					if client, ok := extClientsMap[clientID]; ok {
						if client.DeniedACLs == nil {
							client.DeniedACLs = make(map[string]struct{})
						}
						if acl[acls.AclID(clientID)] == acls.NotAllowed {
							client.DeniedACLs[nodeID] = struct{}{}
						} else {
							delete(client.DeniedACLs, nodeID)
						}
						extClientsMap[clientID] = client
					}
				}
			}
		} else {
			// for client acls
			clientId := string(id)
			for id2 := range acl {
				if _, ok := nodesMap[string(id2)]; !ok {
					// update client acl
					clientId2 := string(id2)
					if client, ok := extClientsMap[clientId]; ok {
						if client.DeniedACLs == nil {
							client.DeniedACLs = make(map[string]struct{})
						}
						{
							// TODO: review this when client-to-client acls are supported
							// if acl[acls.AclID(clientId2)] == acls.NotAllowed {
							// 	client.DeniedACLs[clientId2] = struct{}{}
							// } else {
							// 	delete(client.DeniedACLs, clientId2)
							// }
							delete(client.DeniedACLs, clientId2)
						}
						extClientsMap[clientId] = client
					}
				} else {
					nodeId2 := string(id2)
					if extClientsMap[clientId].IngressGatewayID == nodeId2 && acl[acls.AclID(nodeId2)] == acls.NotAllowed {
						assocClientsToDisconnectPerHost[nodesMap[nodeId2].HostID] = append(assocClientsToDisconnectPerHost[nodesMap[nodeId2].HostID], extClientsMap[clientId])
					}
				}
			}
		}
	}

	// update each client in db for pro servers
	if servercfg.IsPro {
		for _, client := range extClientsMap {
			client := client
			err := logic.DeleteExtClient(client.Network, client.ClientID)
			if err != nil {
				slog.Error(
					"failed to delete client during update",
					"client",
					client.ClientID,
					"error",
					err.Error(),
				)
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
				return
			}
			err = logic.SaveExtClient(&client)
			if err != nil {
				slog.Error(
					"failed to save client during update",
					"client",
					client.ClientID,
					"error",
					err.Error(),
				)
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
				return
			}
		}
	}

	_, err = networkACLUpdateRequest.Save(acls.ContainerID(networkID))
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to update ACLs for network [%s]: %v", networkID, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "updated ACLs for network", networkID)

	logger.Log(1, r.Header.Get("user"), "updated acls for network", networkID)

	// send peer updates
	go func() {
		err = mq.PublishPeerUpdate(false)
		if err != nil {
			logger.Log(0, fmt.Sprintf("failed to publish peer update after network (%s) acl updates: %s", networkID, err.Error()))
		}

		hosts, err := logic.GetAllHosts()
		if err != nil {
			logger.Log(0, fmt.Sprintf("failed to get hosts after network (%s) acl updates: %s", networkID, err.Error()))
			return
		}

		nodes, err := logic.GetAllNodes()
		if err != nil {
			logger.Log(0, fmt.Sprintf("failed to get nodes after network (%s) acl updates: %s", networkID, err.Error()))
			return
		}

		hostsMap := make(map[string]models.Host)
		for _, host := range hosts {
			hostsMap[host.ID.String()] = host
		}

		for hostId, clients := range assocClientsToDisconnectPerHost {
			if host, ok := hostsMap[hostId]; ok {
				if err = mq.PublishSingleHostPeerUpdate(&host, nodes, nil, clients, false, nil); err != nil {
					logger.Log(0, fmt.Sprintf("failed to publish peer update for host (%s) after network (%s) acl updates: %s", networkID, host.ID, err.Error()))
				}
			}
		}
	}()

	logic.ReturnSuccessJsonResponse(w, r, networkACLUpdateRequest)
}

// @Summary     Get a network ACL (Access Control List)
// @Router      /api/networks/{networkname}/acls [get]
// @Tags        Networks
// @Security    oauth
// @Param       networkname path string true "Network name"
// @Produce     json
// @Success     200 {object} acls.ACLContainer
// @Failure     500 {object} models.ErrorResponse
func getNetworkACL(w http.ResponseWriter, r *http.Request) {
	networkID := mux.Vars(r)["networkname"]

	var networkACL acls.ACLContainer
	networkACL, err := networkACL.Get(acls.ContainerID(networkID))
	if err != nil {
		if database.IsEmptyRecord(err) {
			networkACL = acls.ACLContainer{}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(networkACL)
			return
		}
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to fetch ACLs for network [%s]: %v", networkID, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	logic.ReturnSuccessJsonResponse(w, r, networkACL)
}

// @Summary     Get a network Egress routes
// @Router      /api/networks/{networkname}/egress_routes [get]
// @Tags        Networks
// @Security    oauth
// @Param       networkname path string true "Network name"
// @Produce     json
// @Success     200 {object} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func getNetworkEgressRoutes(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	netname := params["networkname"]
	// check if network exists
	_, err := logic.GetNetwork(netname)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to fetch ACLs for network [%s]: %v", netname, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	nodeEgressRoutes, _, err := logic.GetEgressRanges(models.NetworkID(netname))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, nodeEgressRoutes, "fetched network egress routes")
}

// @Summary     Delete a network
// @Router      /api/networks/{networkname} [delete]
// @Tags        Networks
// @Security    oauth
// @Param       networkname path string true "Network name"
// @Param       force query bool false "Force Delete"
// @Produce     json
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     403 {object} models.ErrorResponse
func deleteNetwork(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")
	force := r.URL.Query().Get("force") == "true"
	var params = mux.Vars(r)
	network := params["networkname"]
	doneCh := make(chan struct{}, 1)
	networkNodes, err := logic.GetNetworkNodes(network)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to get network nodes [%s]: %v", network, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	err = logic.DeleteNetwork(network, force, doneCh)
	if err != nil {
		errtype := logic.BadReq
		if strings.Contains(err.Error(), "Node check failed") {
			errtype = logic.Forbidden
		}
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to delete network [%s]: %v", network, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, errtype))
		return
	}
	go logic.UnlinkNetworkAndTagsFromEnrollmentKeys(network, true)
	go logic.DeleteNetworkRoles(network)
	go logic.DeleteAllNetworkTags(models.NetworkID(network))
	go logic.DeleteNetworkPolicies(models.NetworkID(network))
	go func() {
		<-doneCh
		mq.PublishPeerUpdate(true)
		// send node update to clean up locally
		for _, node := range networkNodes {
			node := node
			node.PendingDelete = true
			node.Action = models.NODE_DELETE
			if err := mq.NodeUpdate(&node); err != nil {
				slog.Error("error publishing node update to node", "node", node.ID, "error", err)
			}
		}
		if servercfg.IsDNSMode() {
			logic.SetDNS()
		}
	}()
	logic.LogEvent(&models.Event{
		Action: models.Delete,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: models.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   network,
			Name: network,
			Type: models.NetworkSub,
		},
		Origin: models.Dashboard,
	})
	logger.Log(1, r.Header.Get("user"), "deleted network", network)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode("success")
}

// @Summary     Create a network
// @Router      /api/networks [post]
// @Tags        Networks
// @Security    oauth
// @Param       body body models.Network true "Network details"
// @Produce     json
// @Success     200 {object} models.Network
// @Failure     400 {object} models.ErrorResponse
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

	// validate address ranges: must be private
	if network.AddressRange != "" {
		_, _, err := net.ParseCIDR(network.AddressRange)
		if err != nil {
			logger.Log(0, r.Header.Get("user"), "failed to create network: ",
				err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
	}
	if network.AddressRange6 != "" {
		_, _, err := net.ParseCIDR(network.AddressRange6)
		if err != nil {
			logger.Log(0, r.Header.Get("user"), "failed to create network: ",
				err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
	}

	network, err = logic.CreateNetwork(network)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to create network: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logic.CreateDefaultNetworkRolesAndGroups(models.NetworkID(network.NetID))
	logic.CreateDefaultNetworkPolicies(models.NetworkID(network.NetID))
	logic.CreateDefaultTags(models.NetworkID(network.NetID))

	go func() {
		defaultHosts := logic.GetDefaultHosts()
		for i := range defaultHosts {
			currHost := &defaultHosts[i]
			newNode, err := logic.UpdateHostNetwork(currHost, network.NetID, true)
			if err != nil {
				logger.Log(
					0,
					r.Header.Get("user"),
					"failed to add host to network:",
					currHost.ID.String(),
					network.NetID,
					err.Error(),
				)
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
				return
			}
			logger.Log(1, "added new node", newNode.ID.String(), "to host", currHost.Name)
			if err = mq.HostUpdate(&models.HostUpdate{
				Action: models.JoinHostToNetwork,
				Host:   *currHost,
				Node:   *newNode,
			}); err != nil {
				logger.Log(
					0,
					r.Header.Get("user"),
					"failed to add host to network:",
					currHost.ID.String(),
					network.NetID,
					err.Error(),
				)
			}
			// make  host failover
			logic.CreateFailOver(*newNode)
			// make host remote access gateway
			logic.CreateIngressGateway(network.NetID, newNode.ID.String(), models.IngressRequest{})
			logic.CreateRelay(models.RelayRequest{
				NodeID: newNode.ID.String(),
				NetID:  network.NetID,
			})
		}
		// send peer updates
		if err = mq.PublishPeerUpdate(false); err != nil {
			logger.Log(1, "failed to publish peer update for default hosts after network is added")
		}
	}()
	logic.LogEvent(&models.Event{
		Action: models.Create,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: models.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   network.NetID,
			Name: network.NetID,
			Type: models.NetworkSub,
			Info: network,
		},
		Origin: models.Dashboard,
	})
	logger.Log(1, r.Header.Get("user"), "created network", network.NetID)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(network)
}

// @Summary     Update network settings
// @Router      /api/networks/{networkname} [put]
// @Tags        Networks
// @Security    oauth
// @Param       networkname path string true "Network name"
// @Param       body body models.Network true "Network details"
// @Produce     json
// @Success     200 {object} models.Network
// @Failure     400 {object} models.ErrorResponse
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

	netOld, err := logic.GetNetwork(payload.NetID)
	if err != nil {
		slog.Info("error fetching network", "user", r.Header.Get("user"), "err", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	netNew := netOld
	netNew.NameServers = payload.NameServers
	netNew.DefaultACL = payload.DefaultACL
	_, _, _, err = logic.UpdateNetwork(&netOld, &netNew)
	if err != nil {
		slog.Info("failed to update network", "user", r.Header.Get("user"), "err", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	go mq.PublishPeerUpdate(false)
	slog.Info("updated network", "network", payload.NetID, "user", r.Header.Get("user"))
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(payload)
}
