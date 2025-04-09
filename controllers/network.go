package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gravitl/netmaker/converters"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/gorm"
	"net"
	"net/http"
	"strings"
	"time"

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
	_networks, err := (&schema.Network{}).ListAll(r.Context())
	if err != nil {
		slog.Error("failed to fetch networks", "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	networks := converters.ToModelNetworks(_networks)

	if r.Header.Get("ismaster") != "yes" {
		username := r.Header.Get("user")
		user, err := logic.GetUser(username)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}

		networks = logic.FilterNetworksByRole(networks, *user)
	}

	logger.Log(2, r.Header.Get("user"), "fetched networks.")
	logic.SortNetworks(networks)

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(networks)
}

// @Summary     Lists all networks with stats
// @Router      /api/v1/networks/stats [get]
// @Tags        Networks
// @Security    oauth
// @Produce     json
// @Success     200 {object} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func getNetworksStats(w http.ResponseWriter, r *http.Request) {
	_networks, err := (&schema.Network{}).ListAll(r.Context())
	if err != nil {
		slog.Error("failed to fetch networks", "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	networks := converters.ToModelNetworks(_networks)

	if r.Header.Get("ismaster") != "yes" {
		username := r.Header.Get("user")
		user, err := logic.GetUser(username)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}

		networks = logic.FilterNetworksByRole(networks, *user)
	}

	logic.SortNetworks(networks)

	networkStats := make([]models.NetworkStatResp, len(networks))
	for i, network := range networks {
		_network := &schema.Network{
			ID: network.NetID,
		}
		numNodes, _ := _network.CountNodes(r.Context())

		networkStats[i] = models.NetworkStatResp{
			Network: network,
			Hosts:   numNodes,
		}
	}

	logger.Log(2, r.Header.Get("user"), "fetched networks.")
	logic.ReturnSuccessResponseWithJson(w, r, networkStats, "fetched networks with stats")
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
	networkID := mux.Vars(r)["networkname"]

	_network := &schema.Network{
		ID: networkID,
	}
	err := _network.Get(r.Context())
	if err != nil {
		logger.Log(0, r.Header.Get("user"), fmt.Sprintf("failed to fetch network [%s] info: %v",
			networkID, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	logger.Log(2, r.Header.Get("user"), "fetched network", networkID)

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(converters.ToModelNetwork(*_network))
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
	w.Header().Set("Content-Type", "application/json")

	networkID := mux.Vars(r)["networkname"]

	var networkACLChange acls.ACLContainer
	networkACLChange, err := networkACLChange.Get(acls.ContainerID(networkID))
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to fetch ACLs for network [%s]: %v", networkID, err))
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

	newNetACL, err := networkACLChange.Save(acls.ContainerID(networkID))
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

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(newNetACL)
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
	w.Header().Set("Content-Type", "application/json")

	networkID := mux.Vars(r)["networkname"]

	var networkACLChange acls.ACLContainer
	networkACLChange, err := networkACLChange.Get(acls.ContainerID(networkID))
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to fetch ACLs for network [%s]: %v", networkID, err))
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

	// clone req body to use as return data successful update
	retData := make(acls.ACLContainer)
	data, err := json.Marshal(networkACLChange)
	if err != nil {
		slog.Error("failed to marshal networkACLChange whiles cloning", "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	err = json.Unmarshal(data, &retData)
	if err != nil {
		slog.Error("failed to unmarshal networkACLChange whiles cloning", "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	_network := &schema.Network{
		ID: networkID,
	}
	err = _network.GetNodes(r.Context())
	if err != nil {
		slog.Error("failed to fetch network nodes", "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	networkNodesIdMap := make(map[string]schema.Node)
	for _, node := range _network.Nodes {
		networkNodesIdMap[node.ID] = node
	}

	networkClients, err := logic.GetNetworkExtClients(networkID)
	if err != nil {
		slog.Error("failed to fetch network clients", "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	networkClientsMap := make(map[string]models.ExtClient)
	for _, client := range networkClients {
		networkClientsMap[client.ClientID] = client
	}

	// keep track of ingress gateways to disconnect from their clients
	// this is required because PublishPeerUpdate only somehow does not stop communication
	// between blocked clients and their ingress
	assocClientsToDisconnectPerHost := make(map[string][]models.ExtClient)

	// update client acls and then, remove client acls from req data to pass to existing functions
	for id, acl := range networkACLChange {
		// for node acls
		if _, ok := networkNodesIdMap[string(id)]; ok {
			nodeId := string(id)
			// check acl update, then remove client entries
			for id2 := range acl {
				if _, ok := networkNodesIdMap[string(id2)]; !ok {
					// update client acl
					clientId := string(id2)
					if client, ok := networkClientsMap[clientId]; ok {
						if client.DeniedACLs == nil {
							client.DeniedACLs = make(map[string]struct{})
						}
						if acl[acls.AclID(clientId)] == acls.NotAllowed {
							client.DeniedACLs[nodeId] = struct{}{}
						} else {
							delete(client.DeniedACLs, string(nodeId))
						}
						networkClientsMap[clientId] = client
					}
				}
			}
		} else {
			// for client acls
			clientId := string(id)
			for id2 := range acl {
				if _, ok := networkNodesIdMap[string(id2)]; !ok {
					// update client acl
					clientId2 := string(id2)
					if client, ok := networkClientsMap[clientId]; ok {
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
						networkClientsMap[clientId] = client
					}
				} else {
					nodeId2 := string(id2)
					if networkClientsMap[clientId].IngressGatewayID == nodeId2 && acl[acls.AclID(nodeId2)] == acls.NotAllowed {
						assocClientsToDisconnectPerHost[networkNodesIdMap[nodeId2].HostID] = append(assocClientsToDisconnectPerHost[networkNodesIdMap[nodeId2].HostID], networkClientsMap[clientId])
					}
				}
			}
		}
	}

	// update each client in db for pro servers
	if servercfg.IsPro {
		for _, client := range networkClientsMap {
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

	_, err = networkACLChange.Save(acls.ContainerID(networkID))
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

		// update ingress gateways of associated clients
		hosts, err := logic.GetAllHosts()
		if err != nil {
			slog.Error(
				"failed to fetch hosts after network ACL update. skipping publish extclients ACL",
				"network",
				networkID,
			)
			return
		}

		hostsMap := make(map[string]models.Host)
		for _, host := range hosts {
			hostsMap[host.ID.String()] = host
		}

		for hostId, clients := range assocClientsToDisconnectPerHost {
			if host, ok := hostsMap[hostId]; ok {
				if err = mq.PublishSingleHostPeerUpdate(&host, nil, clients, false, nil); err != nil {
					slog.Error("failed to publish peer update to ingress after ACL update on network", "network", networkID, "host", hostId)
				}
			}
		}
	}()

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(networkACLChange)
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
	w.Header().Set("Content-Type", "application/json")

	networkID := mux.Vars(r)["networkname"]

	var networkACL acls.ACLContainer
	networkACL, err := networkACL.Get(acls.ContainerID(networkID))
	if err != nil {
		if database.IsEmptyRecord(err) {
			networkACL = acls.ACLContainer{}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(networkACL)
			return
		}
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to fetch ACLs for network [%s]: %v", networkID, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	logger.Log(2, r.Header.Get("user"), "fetched acl for network", networkID)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(networkACL)
}

// @Summary     Get a network Egress routes
// @Router      /api/networks/{networkname}/egress_routes [get]
// @Tags        Networks
// @Security    oauth
// @Param       networkname path string true "Network name"
// @Produce     json
// @Success     200 {object} acls.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func getNetworkEgressRoutes(w http.ResponseWriter, r *http.Request) {
	networkID := mux.Vars(r)["networkname"]

	_network := &schema.Network{
		ID: networkID,
	}
	err := _network.Get(r.Context())
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to fetch ACLs for network [%s]: %v", networkID, err))
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		} else {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		}
		return
	}

	nodeEgressRoutes, _, err := logic.GetEgressRanges(models.NetworkID(networkID))
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

	networkID := mux.Vars(r)["networkname"]
	doneCh := make(chan struct{}, 1)

	_network := &schema.Network{
		ID: networkID,
	}
	err := _network.GetNodes(r.Context())
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to get network nodes [%s]: %v", networkID, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	err = logic.DeleteNetwork(networkID, force, doneCh)
	if err != nil {
		errtype := "badrequest"
		if strings.Contains(err.Error(), "Node check failed") {
			errtype = "forbidden"
		}
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to delete network [%s]: %v", networkID, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, errtype))
		return
	}
	go logic.UnlinkNetworkAndTagsFromEnrollmentKeys(networkID, true)
	go logic.DeleteNetworkRoles(networkID)
	go logic.DeleteDefaultNetworkPolicies(models.NetworkID(networkID))

	go func() {
		<-doneCh
		_ = mq.PublishPeerUpdate(true)
		// send node update to clean up locally
		for _, _node := range _network.Nodes {
			node := converters.ToModelNode(_node)
			node.PendingDelete = true
			node.Action = models.NODE_DELETE
			if err := mq.NodeUpdate(&node); err != nil {
				slog.Error("error publishing node update to node", "node", node.ID, "error", err)
			}
		}
		if servercfg.IsDNSMode() {
			_ = logic.SetDNS()
		}
	}()

	logger.Log(1, r.Header.Get("user"), "deleted network", networkID)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode("success")
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
	logic.CreateDefaultAclNetworkPolicies(models.NetworkID(network.NetID))
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
		}
		// send peer updates
		if err = mq.PublishPeerUpdate(false); err != nil {
			logger.Log(1, "failed to publish peer update for default hosts after network is added")
		}
	}()

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

	networkID := mux.Vars(r)["networkname"]

	var networkChanges models.Network
	err := json.NewDecoder(r.Body).Decode(&networkChanges)
	if err != nil {
		slog.Info("error decoding request body", "user", r.Header.Get("user"), "err", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	if networkChanges.NetID != networkID {
		slog.Info("mismatch between network id in body and path param", "user", r.Header.Get("user"))
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("mismatch between network id in body and path param"),
				"badrequest",
			),
		)
		return
	}

	_network := &schema.Network{
		ID: networkID,
	}
	err = _network.Get(r.Context())
	if err != nil {
		slog.Info("error fetching network", "user", r.Header.Get("user"), "err", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	_network.NameServers = networkChanges.NameServers
	_network.DefaultACL = networkChanges.DefaultACL
	_network.NetworkLastModified = time.Now()
	err = _network.Update(r.Context())
	if err != nil {
		slog.Info("failed to update network", "user", r.Header.Get("user"), "err", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	go mq.PublishPeerUpdate(false)

	slog.Info("updated network", "network", networkChanges.NetID, "user", r.Header.Get("user"))
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(converters.ToModelNetwork(*_network))
}
