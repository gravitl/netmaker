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
	"golang.org/x/exp/slog"
)

func UserHandlers(r *mux.Router) {
	r.HandleFunc("/api/users/{username}/remote_access_gw/{remote_access_gateway_id}", logic.SecurityCheck(true, http.HandlerFunc(attachUserToRemoteAccessGw))).Methods(http.MethodPost)
	r.HandleFunc("/api/users/{username}/remote_access_gw/{remote_access_gateway_id}", logic.SecurityCheck(true, http.HandlerFunc(removeUserFromRemoteAccessGW))).Methods(http.MethodDelete)
	r.HandleFunc("/api/users/{username}/remote_access_gw", logic.SecurityCheck(false, logic.ContinueIfUserMatch(http.HandlerFunc(getUserRemoteAccessGws)))).Methods(http.MethodGet)
	r.HandleFunc("/api/users/ingress/{ingress_id}", logic.SecurityCheck(true, http.HandlerFunc(ingressGatewayUsers))).Methods(http.MethodGet)
}

// swagger:route POST /api/users/{username}/remote_access_gw user attachUserToRemoteAccessGateway
//
// Attach User to a remote access gateway.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
func attachUserToRemoteAccessGw(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	username := params["username"]
	remoteGwID := params["remote_access_gateway_id"]
	if username == "" || remoteGwID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("required params `username` and `remote_access_gateway_id`"), "badrequest"))
		return
	}
	user, err := logic.GetUser(username)
	if err != nil {
		slog.Error("failed to fetch user: ", "username", username, "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to fetch user %s, error: %v", username, err), "badrequest"))
		return
	}
	if user.IsAdmin || user.IsSuperAdmin {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("superadmins/admins have access to all gateways"), "badrequest"))
		return
	}
	node, err := logic.GetNodeByID(remoteGwID)
	if err != nil {
		slog.Error("failed to fetch gateway node", "nodeID", remoteGwID, "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to fetch remote access gateway node, error: %v", err), "badrequest"))
		return
	}
	if !node.IsIngressGateway {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("node is not a remote access gateway"), "badrequest"))
		return
	}
	if user.RemoteGwIDs == nil {
		user.RemoteGwIDs = make(map[string]struct{})
	}
	user.RemoteGwIDs[node.ID.String()] = struct{}{}
	err = logic.UpsertUser(*user)
	if err != nil {
		slog.Error("failed to update user's gateways", "user", username, "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to fetch remote access gateway node,error: %v", err), "badrequest"))
		return
	}

	json.NewEncoder(w).Encode(logic.ToReturnUser(*user))
}

// swagger:route DELETE /api/users/{username}/remote_access_gw user removeUserFromRemoteAccessGW
//
// Delete User from a remote access gateway.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
func removeUserFromRemoteAccessGW(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	username := params["username"]
	remoteGwID := params["remote_access_gateway_id"]
	if username == "" || remoteGwID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("required params `username` and `remote_access_gateway_id`"), "badrequest"))
		return
	}
	user, err := logic.GetUser(username)
	if err != nil {
		logger.Log(0, username, "failed to fetch user: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to fetch user %s, error: %v", username, err), "badrequest"))
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
				logic.DeleteExtClient(extclient.Network, extclient.ClientID)
			}
		}
	}(*user, remoteGwID)

	err = logic.UpsertUser(*user)
	if err != nil {
		slog.Error("failed to update user gateways", "user", username, "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("failed to fetch remote access gaetway node "+err.Error()), "badrequest"))
		return
	}
	json.NewEncoder(w).Encode(logic.ToReturnUser(*user))
}

// swagger:route GET "/api/users/{username}/remote_access_gw" nodes getUserRemoteAccessGws
//
// Get an individual node.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeResponse
func getUserRemoteAccessGws(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	username := params["username"]
	if username == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("required params username"), "badrequest"))
		return
	}
	var req models.UserRemoteGwsReq
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		slog.Error("error decoding request body: ", "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if req.RemoteAccessClientID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("remote access client id cannot be empty"), "badrequest"))
		return
	}
	userGws := make(map[string][]models.UserRemoteGws)
	user, err := logic.GetUser(username)
	if err != nil {
		logger.Log(0, username, "failed to fetch user: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to fetch user %s, error: %v", username, err), "badrequest"))
		return
	}
	if user.IsAdmin || user.IsSuperAdmin {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("admins can visit dashboard to create remote clients"), "badrequest"))
		return
	}
	allextClients, err := logic.GetAllExtClients()
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	for _, extClient := range allextClients {
		if extClient.RemoteAccessClientID == req.RemoteAccessClientID && extClient.OwnerID == username {
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

			if _, ok := user.RemoteGwIDs[node.ID.String()]; ok {
				gws := userGws[node.Network]
				extClient.AllowedIPs = logic.GetExtclientAllowedIPs(extClient)
				gws = append(gws, models.UserRemoteGws{
					GwID:              node.ID.String(),
					GWName:            host.Name,
					Network:           node.Network,
					GwClient:          extClient,
					Connected:         true,
					IsInternetGateway: node.IsInternetGateway,
				})
				userGws[node.Network] = gws
				delete(user.RemoteGwIDs, node.ID.String())

			}
		}

	}

	// add remaining gw nodes to resp
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
		gws := userGws[node.Network]

		gws = append(gws, models.UserRemoteGws{
			GwID:              node.ID.String(),
			GWName:            host.Name,
			Network:           node.Network,
			IsInternetGateway: node.IsInternetGateway,
		})
		userGws[node.Network] = gws
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(userGws)
}

// swagger:route GET /api/nodes/{network}/{nodeid}/ingress/users users ingressGatewayUsers
//
// Lists all the users attached to an ingress gateway.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeResponse
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
		slog.Error("failed to get users on ingress gateway", "nodeid", ingressID, "network", node.Network, "user", r.Header.Get("user"),
			"error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(gwUsers)
}
