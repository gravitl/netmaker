package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/pro/auth"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slog"
)

func UserHandlers(r *mux.Router) {
	r.HandleFunc("/api/users/{username}/remote_access_gw/{remote_access_gateway_id}", logic.SecurityCheck(true, http.HandlerFunc(attachUserToRemoteAccessGw))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/users/{username}/remote_access_gw/{remote_access_gateway_id}", logic.SecurityCheck(true, http.HandlerFunc(removeUserFromRemoteAccessGW))).
		Methods(http.MethodDelete)
	r.HandleFunc("/api/users/{username}/remote_access_gw", logic.SecurityCheck(false, logic.ContinueIfUserMatch(http.HandlerFunc(getUserRemoteAccessGws)))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/users/ingress/{ingress_id}", logic.SecurityCheck(true, http.HandlerFunc(ingressGatewayUsers))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/oauth/login", auth.HandleAuthLogin).Methods(http.MethodGet)
	r.HandleFunc("/api/oauth/callback", auth.HandleAuthCallback).Methods(http.MethodGet)
	r.HandleFunc("/api/oauth/headless", auth.HandleHeadlessSSO)
	r.HandleFunc("/api/oauth/register/{regKey}", auth.RegisterHostSSO).Methods(http.MethodGet)
}

// @Summary     Attach user to a remote access gateway
// @Router      /api/users/{username}/remote_access_gw/{remote_access_gateway_id} [post]
// @Tags        PRO
// @Accept      json
// @Produce     json
// @Param       username path string true "Username"
// @Param       remote_access_gateway_id path string true "Remote Access Gateway ID"
// @Success     200 {object} models.ReturnUser
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func attachUserToRemoteAccessGw(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	username := params["username"]
	remoteGwID := params["remote_access_gateway_id"]
	if username == "" || remoteGwID == "" {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				errors.New("required params `username` and `remote_access_gateway_id`"),
				"badrequest",
			),
		)
		return
	}
	user, err := logic.GetUser(username)
	if err != nil {
		slog.Error("failed to fetch user: ", "username", username, "error", err.Error())
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				fmt.Errorf("failed to fetch user %s, error: %v", username, err),
				"badrequest",
			),
		)
		return
	}
	if user.IsAdmin || user.IsSuperAdmin {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				errors.New("superadmins/admins have access to all gateways"),
				"badrequest",
			),
		)
		return
	}
	node, err := logic.GetNodeByID(remoteGwID)
	if err != nil {
		slog.Error("failed to fetch gateway node", "nodeID", remoteGwID, "error", err)
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				fmt.Errorf("failed to fetch remote access gateway node, error: %v", err),
				"badrequest",
			),
		)
		return
	}
	if !node.IsIngressGateway {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(fmt.Errorf("node is not a remote access gateway"), "badrequest"),
		)
		return
	}
	if user.RemoteGwIDs == nil {
		user.RemoteGwIDs = make(map[string]struct{})
	}
	user.RemoteGwIDs[node.ID.String()] = struct{}{}
	err = logic.UpsertUser(*user)
	if err != nil {
		slog.Error("failed to update user's gateways", "user", username, "error", err)
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				fmt.Errorf("failed to fetch remote access gateway node,error: %v", err),
				"badrequest",
			),
		)
		return
	}

	json.NewEncoder(w).Encode(logic.ToReturnUser(*user))
}

// @Summary     Remove user from a remote access gateway
// @Router      /api/users/{username}/remote_access_gw/{remote_access_gateway_id} [delete]
// @Tags        PRO
// @Accept      json
// @Produce     json
// @Param       username path string true "Username"
// @Param       remote_access_gateway_id path string true "Remote Access Gateway ID"
// @Success     200 {object} models.ReturnUser
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func removeUserFromRemoteAccessGW(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	username := params["username"]
	remoteGwID := params["remote_access_gateway_id"]
	if username == "" || remoteGwID == "" {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				errors.New("required params `username` and `remote_access_gateway_id`"),
				"badrequest",
			),
		)
		return
	}
	user, err := logic.GetUser(username)
	if err != nil {
		logger.Log(0, username, "failed to fetch user: ", err.Error())
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				fmt.Errorf("failed to fetch user %s, error: %v", username, err),
				"badrequest",
			),
		)
		return
	}
	delete(user.RemoteGwIDs, remoteGwID)
	go func(user models.User, remoteGwID string) {
		extclients, err := logic.GetAllExtClients()
		if err != nil {
			slog.Error("failed to fetch extclients", "error", err)
			return
		}
		for _, extclient := range extclients {
			if extclient.OwnerID == user.UserName && remoteGwID == extclient.IngressGatewayID {
				err = logic.DeleteExtClientAndCleanup(extclient)
				if err != nil {
					slog.Error("failed to delete extclient",
						"id", extclient.ClientID, "owner", user.UserName, "error", err)
				} else {
					if err := mq.PublishDeletedClientPeerUpdate(&extclient); err != nil {
						slog.Error("error setting ext peers: " + err.Error())
					}
				}
			}
		}
		if servercfg.IsDNSMode() {
			logic.SetDNS()
		}
	}(*user, remoteGwID)

	err = logic.UpsertUser(*user)
	if err != nil {
		slog.Error("failed to update user gateways", "user", username, "error", err)
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				errors.New("failed to fetch remote access gaetway node "+err.Error()),
				"badrequest",
			),
		)
		return
	}
	json.NewEncoder(w).Encode(logic.ToReturnUser(*user))
}

// @Summary     Get user's remote access gateways
// @Router      /api/users/{username}/remote_access_gw [get]
// @Tags        PRO
// @Accept      json
// @Produce     json
// @Param       username path string true "Username"
// @Param       remote_access_clientid query string false "Remote Access Client ID"
// @Param       from_mobile query boolean false "Request from mobile"
// @Success     200 {array} models.UserRemoteGws
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func getUserRemoteAccessGws(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	username := params["username"]
	if username == "" {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("required params username"), "badrequest"),
		)
		return
	}
	remoteAccessClientID := r.URL.Query().Get("remote_access_clientid")
	var req models.UserRemoteGwsReq
	if remoteAccessClientID == "" {
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			slog.Error("error decoding request body: ", "error", err)
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
	}
	reqFromMobile := r.URL.Query().Get("from_mobile") == "true"
	if req.RemoteAccessClientID == "" && remoteAccessClientID == "" {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("remote access client id cannot be empty"), "badrequest"),
		)
		return
	}
	if req.RemoteAccessClientID == "" {
		req.RemoteAccessClientID = remoteAccessClientID
	}
	userGws := make(map[string][]models.UserRemoteGws)
	user, err := logic.GetUser(username)
	if err != nil {
		logger.Log(0, username, "failed to fetch user: ", err.Error())
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				fmt.Errorf("failed to fetch user %s, error: %v", username, err),
				"badrequest",
			),
		)
		return
	}
	allextClients, err := logic.GetAllExtClients()
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	processedAdminNodeIds := make(map[string]struct{})
	for _, extClient := range allextClients {
		if extClient.RemoteAccessClientID == req.RemoteAccessClientID &&
			extClient.OwnerID == username {
			node, err := logic.GetNodeByID(extClient.IngressGatewayID)
			if err != nil {
				continue
			}
			if node.PendingDelete {
				continue
			}
			if !node.IsIngressGateway {
				continue
			}
			host, err := logic.GetHost(node.HostID.String())
			if err != nil {
				continue
			}
			network, err := logic.GetNetwork(node.Network)
			if err != nil {
				slog.Error("failed to get node network", "error", err)
			}

			if _, ok := user.RemoteGwIDs[node.ID.String()]; (!user.IsAdmin && !user.IsSuperAdmin) &&
				ok {
				gws := userGws[node.Network]
				extClient.AllowedIPs = logic.GetExtclientAllowedIPs(extClient)
				gws = append(gws, models.UserRemoteGws{
					GwID:              node.ID.String(),
					GWName:            host.Name,
					Network:           node.Network,
					GwClient:          extClient,
					Connected:         true,
					IsInternetGateway: node.IsInternetGateway,
					GwPeerPublicKey:   host.PublicKey.String(),
					GwListenPort:      logic.GetPeerListenPort(host),
					Metadata:          node.Metadata,
					AllowedEndpoints:  getAllowedRagEndpoints(&node, host),
					NetworkAddresses:  []string{network.AddressRange, network.AddressRange6},
				})
				userGws[node.Network] = gws
				delete(user.RemoteGwIDs, node.ID.String())
			} else {
				gws := userGws[node.Network]
				extClient.AllowedIPs = logic.GetExtclientAllowedIPs(extClient)
				gws = append(gws, models.UserRemoteGws{
					GwID:              node.ID.String(),
					GWName:            host.Name,
					Network:           node.Network,
					GwClient:          extClient,
					Connected:         true,
					IsInternetGateway: node.IsInternetGateway,
					GwPeerPublicKey:   host.PublicKey.String(),
					GwListenPort:      logic.GetPeerListenPort(host),
					Metadata:          node.Metadata,
					AllowedEndpoints:  getAllowedRagEndpoints(&node, host),
					NetworkAddresses:  []string{network.AddressRange, network.AddressRange6},
				})
				userGws[node.Network] = gws
				processedAdminNodeIds[node.ID.String()] = struct{}{}
			}
		}
	}

	// add remaining gw nodes to resp
	if !user.IsAdmin && !user.IsSuperAdmin {
		for gwID := range user.RemoteGwIDs {
			node, err := logic.GetNodeByID(gwID)
			if err != nil {
				continue
			}
			if !node.IsIngressGateway {
				continue
			}
			if node.PendingDelete {
				continue
			}
			host, err := logic.GetHost(node.HostID.String())
			if err != nil {
				continue
			}
			network, err := logic.GetNetwork(node.Network)
			if err != nil {
				slog.Error("failed to get node network", "error", err)
			}
			gws := userGws[node.Network]

			gws = append(gws, models.UserRemoteGws{
				GwID:              node.ID.String(),
				GWName:            host.Name,
				Network:           node.Network,
				IsInternetGateway: node.IsInternetGateway,
				GwPeerPublicKey:   host.PublicKey.String(),
				GwListenPort:      logic.GetPeerListenPort(host),
				Metadata:          node.Metadata,
				AllowedEndpoints:  getAllowedRagEndpoints(&node, host),
				NetworkAddresses:  []string{network.AddressRange, network.AddressRange6},
			})
			userGws[node.Network] = gws
		}
	} else {
		allNodes, err := logic.GetAllNodes()
		if err != nil {
			slog.Error("failed to fetch all nodes", "error", err)
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
		for _, node := range allNodes {
			_, ok := processedAdminNodeIds[node.ID.String()]
			if node.IsIngressGateway && !node.PendingDelete && !ok {
				host, err := logic.GetHost(node.HostID.String())
				if err != nil {
					slog.Error("failed to fetch host", "error", err)
					continue
				}
				network, err := logic.GetNetwork(node.Network)
				if err != nil {
					slog.Error("failed to get node network", "error", err)
				}
				gws := userGws[node.Network]

				gws = append(gws, models.UserRemoteGws{
					GwID:              node.ID.String(),
					GWName:            host.Name,
					Network:           node.Network,
					IsInternetGateway: node.IsInternetGateway,
					GwPeerPublicKey:   host.PublicKey.String(),
					GwListenPort:      logic.GetPeerListenPort(host),
					Metadata:          node.Metadata,
					AllowedEndpoints:  getAllowedRagEndpoints(&node, host),
					NetworkAddresses:  []string{network.AddressRange, network.AddressRange6},
				})
				userGws[node.Network] = gws
			}
		}
	}
	if reqFromMobile {
		// send resp in array format
		userGwsArr := []models.UserRemoteGws{}
		for _, userGwI := range userGws {
			userGwsArr = append(userGwsArr, userGwI...)
		}
		logic.ReturnSuccessResponseWithJson(w, r, userGwsArr, "fetched gateways for user"+username)
		return
	}
	slog.Debug("returned user gws", "user", username, "gws", userGws)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(userGws)
}

// @Summary     List users attached to an ingress gateway
// @Router      /api/nodes/{network}/{nodeid}/ingress/users [get]
// @Tags        PRO
// @Accept      json
// @Produce     json
// @Param       ingress_id path string true "Ingress Gateway ID"
// @Success     200 {array} models.IngressGwUsers
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func ingressGatewayUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	ingressID := params["ingress_id"]
	node, err := logic.GetNodeByID(ingressID)
	if err != nil {
		slog.Error("failed to get ingress node", "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	gwUsers, err := logic.GetIngressGwUsers(node)
	if err != nil {
		slog.Error(
			"failed to get users on ingress gateway",
			"nodeid",
			ingressID,
			"network",
			node.Network,
			"user",
			r.Header.Get("user"),
			"error",
			err,
		)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(gwUsers)
}

func getAllowedRagEndpoints(ragNode *models.Node, ragHost *models.Host) []string {
	endpoints := []string{}
	if len(ragHost.EndpointIP) > 0 {
		endpoints = append(endpoints, ragHost.EndpointIP.String())
	}
	if len(ragHost.EndpointIPv6) > 0 {
		endpoints = append(endpoints, ragHost.EndpointIPv6.String())
	}
	if servercfg.IsPro {
		for _, ip := range ragNode.AdditionalRagIps {
			endpoints = append(endpoints, ip.String())
		}
	}
	return endpoints
}
