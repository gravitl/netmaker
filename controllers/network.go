package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/schema"
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
	r.HandleFunc("/api/networks", logic.SecurityCheck(true, http.HandlerFunc(createNetwork))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/networks/{networkname}", logic.SecurityCheck(true, http.HandlerFunc(getNetwork))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/networks/{networkname}", logic.SecurityCheck(true, http.HandlerFunc(deleteNetwork))).
		Methods(http.MethodDelete)
	r.HandleFunc("/api/networks/{networkname}", logic.SecurityCheck(true, http.HandlerFunc(updateNetwork))).
		Methods(http.MethodPut)
	r.HandleFunc("/api/networks/{networkname}/egress_routes", logic.SecurityCheck(true, http.HandlerFunc(getNetworkEgressRoutes)))
}

// @Summary     Lists all networks
// @Router      /api/networks [get]
// @Tags        Networks
// @Security    oauth
// @Produce     json
// @Success     200 {array} schema.Network
// @Failure     500 {object} models.ErrorResponse
func getNetworks(w http.ResponseWriter, r *http.Request) {
	var err error

	allnetworks, err := (&schema.Network{}).ListAll(r.Context())
	if err != nil {
		slog.Error("failed to fetch networks", "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if r.Header.Get("ismaster") != "yes" {
		username := r.Header.Get("user")
		user := &schema.User{Username: username}
		err = user.Get(r.Context())
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
		allnetworks = logic.FilterNetworksByRole(allnetworks, user)
	}

	logger.Log(2, r.Header.Get("user"), "fetched networks.")
	logic.SortNetworks(allnetworks[:])
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(allnetworks)
}

// @Summary     Lists all networks with stats
// @Router      /api/v1/networks/stats [get]
// @Tags        Networks
// @Security    oauth
// @Produce     json
// @Success     200 {array} models.NetworkStatResp
// @Failure     500 {object} models.ErrorResponse
func getNetworksStats(w http.ResponseWriter, r *http.Request) {

	var err error
	allnetworks, err := (&schema.Network{}).ListAll(r.Context())
	if err != nil {
		slog.Error("failed to fetch networks", "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if r.Header.Get("ismaster") != "yes" {
		username := r.Header.Get("user")
		user := &schema.User{Username: username}
		err = user.Get(r.Context())
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
		allnetworks = logic.FilterNetworksByRole(allnetworks, user)
	}
	allNodes, err := logic.GetAllNodes()
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	type networkStatResp struct {
		Network *schema.Network
		Hosts   int
	}

	var netstats []networkStatResp
	logic.SortNetworks(allnetworks[:])
	for _, network := range allnetworks {
		netstats = append(netstats, networkStatResp{
			Network: &network,
			Hosts:   len(logic.GetNetworkNodesMemory(allNodes, network.Name)),
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
// @Success     200 {object} schema.Network
// @Failure     404 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func getNetwork(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	netname := params["networkname"]
	network := &schema.Network{Name: netname}
	err := network.Get(r.Context())
	if err != nil {
		logger.Log(0, r.Header.Get("user"), fmt.Sprintf("failed to fetch network [%s] info: %v",
			netname, err))

		errType := logic.Internal
		if database.IsEmptyRecord(err) {
			errType = logic.NotFound
		}

		logic.ReturnErrorResponse(w, r, logic.FormatError(err, errType))
		return
	}

	logger.Log(2, r.Header.Get("user"), "fetched network", netname)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(network)
}

// @Summary     Get a network Egress routes
// @Router      /api/networks/{networkname}/egress_routes [get]
// @Tags        Networks
// @Security    oauth
// @Param       networkname path string true "Network name"
// @Produce     json
// @Success     200 {object} map[string][]string
// @Failure     500 {object} models.ErrorResponse
func getNetworkEgressRoutes(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	netname := params["networkname"]
	// check if network exists
	err := (&schema.Network{Name: netname}).Get(r.Context())
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to fetch ACLs for network [%s]: %v", netname, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	nodeEgressRoutes, _, err := logic.GetEgressRanges(schema.NetworkID(netname))
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
	go logic.DeleteAllNetworkTags(schema.NetworkID(network))
	go logic.DeleteNetworkPolicies(schema.NetworkID(network))
	//delete network from allocated ip map
	go logic.RemoveNetworkFromAllocatedIpMap(network)
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

		_ = logic.DeleteNetworkNameservers(network)
		if servercfg.IsDNSMode() {
			logic.SetDNS()
		}
	}()
	logic.LogEvent(&models.Event{
		Action: schema.Delete,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   network,
			Name: network,
			Type: schema.NetworkSub,
		},
		Origin: schema.Dashboard,
		Diff: models.Diff{
			Old: network,
			New: nil,
		},
	})
	logger.Log(1, r.Header.Get("user"), "deleted network", network)
	w.WriteHeader(http.StatusOK)
	logic.ReturnSuccessResponse(w, r, "success")
}

// @Summary     Create a network
// @Router      /api/networks [post]
// @Tags        Networks
// @Security    oauth
// @Param       body body schema.Network true "Network details"
// @Produce     json
// @Success     200 {object} schema.Network
// @Failure     400 {object} models.ErrorResponse
func createNetwork(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var network schema.Network

	// we decode our body request params
	err := json.NewDecoder(r.Body).Decode(&network)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	featureFlags := logic.GetFeatureFlags()
	if !featureFlags.EnableDeviceApproval {
		network.AutoJoin = true
	}
	if len(network.Name) > 32 {
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
		_, cidr, err := net.ParseCIDR(network.AddressRange)
		if err != nil {
			logger.Log(0, r.Header.Get("user"), "failed to create network: ",
				err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		} else {
			ones, bits := cidr.Mask.Size()
			if bits-ones <= 1 {
				err = fmt.Errorf("cannot create network with /31 or /32 cidr")
				logger.Log(0, r.Header.Get("user"), "failed to create network: ",
					err.Error())
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
				return
			}
		}
	}

	if network.AddressRange6 != "" {
		_, cidr, err := net.ParseCIDR(network.AddressRange6)
		if err != nil {
			logger.Log(0, r.Header.Get("user"), "failed to create network: ",
				err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		} else {
			ones, bits := cidr.Mask.Size()
			if bits-ones <= 1 {
				err = fmt.Errorf("cannot create network with /127 or /128 cidr")
				logger.Log(0, r.Header.Get("user"), "failed to create network: ",
					err.Error())
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
				return
			}
		}
	}
	if network.AutoRemove {
		if network.AutoRemoveThreshold == 0 {
			network.AutoRemoveThreshold = 60
		}
	}
	if network.AutoRemoveTags == nil {
		network.AutoRemoveTags = []string{}
	}
	err = logic.CreateNetwork(&network)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to create network: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logic.CreateDefaultNetworkRolesAndGroups(schema.NetworkID(network.Name))
	logic.CreateDefaultAclNetworkPolicies(schema.NetworkID(network.Name))
	logic.CreateDefaultTags(schema.NetworkID(network.Name))
	logic.AddNetworkToAllocatedIpMap(network.Name)
	logic.CreateFallbackNameserver(network.Name)
	if featureFlags.EnableOverlappingEgressRanges {
		if err := logic.AllocateUniqueVNATPool(&network); err != nil {
			logger.Log(0, r.Header.Get("user"), "failed to allocate unique virtual NAT pool:", err.Error())
		} else if err := logic.UpsertNetwork(&network); err != nil {
			logger.Log(0, r.Header.Get("user"), "failed to update network with virtual NAT settings:", err.Error())
		}
	}
	go func() {
		defaultHosts := logic.GetDefaultHosts()
		for i := range defaultHosts {
			currHost := &defaultHosts[i]
			newNode, err := logic.UpdateHostNetwork(currHost, network.Name, true)
			if err != nil {
				logger.Log(
					0,
					r.Header.Get("user"),
					"failed to add host to network:",
					currHost.ID.String(),
					network.Name,
					err.Error(),
				)
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
				return
			}
			logger.Log(1, "added new node", newNode.ID.String(), "to host", currHost.Name)
			if len(currHost.Nodes) == 1 {
				if err = mq.HostUpdate(&models.HostUpdate{
					Action: models.RequestPull,
					Host:   *currHost,
					Node:   *newNode,
				}); err != nil {
					logger.Log(
						0,
						r.Header.Get("user"),
						"failed to add host to network:",
						currHost.ID.String(),
						network.Name,
						err.Error(),
					)
				}
			} else {
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
						network.Name,
						err.Error(),
					)
				}
			}

			// make  host failover
			logic.CreateFailOver(*newNode)
			// make host remote access gateway
			logic.CreateIngressGateway(network.Name, newNode.ID.String(), models.IngressRequest{})
			logic.CreateRelay(models.RelayRequest{
				NodeID: newNode.ID.String(),
				NetID:  network.Name,
			})
		}
		// send peer updates
		if err = mq.PublishPeerUpdate(false); err != nil {
			logger.Log(1, "failed to publish peer update for default hosts after network is added")
		}
	}()
	logic.LogEvent(&models.Event{
		Action: schema.Create,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   network.Name,
			Name: network.Name,
			Type: schema.NetworkSub,
			Info: network,
		},
		Origin: schema.Dashboard,
	})
	logger.Log(1, r.Header.Get("user"), "created network", network.Name)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(network)
}

// @Summary     Update network settings
// @Router      /api/networks/{networkname} [put]
// @Tags        Networks
// @Security    oauth
// @Param       networkname path string true "Network name"
// @Param       body body schema.Network true "Network details"
// @Produce     json
// @Success     200 {object} schema.Network
// @Failure     400 {object} models.ErrorResponse
func updateNetwork(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var payload schema.Network

	// we decode our body request params
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		slog.Info("error decoding request body", "user", r.Header.Get("user"), "err", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	currNet := &schema.Network{Name: payload.Name}
	err = currNet.Get(r.Context())
	if err != nil {
		slog.Info("error fetching network", "user", r.Header.Get("user"), "err", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	err = logic.UpdateNetwork(currNet, &payload)
	if err != nil {
		slog.Info("failed to update network", "user", r.Header.Get("user"), "err", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	go mq.PublishPeerUpdate(false)
	slog.Info("updated network", "network", payload.Name, "user", r.Header.Get("user"))
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(currNet)
}
