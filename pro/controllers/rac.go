package controllers

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logic"
)

func RacHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/rac/networks", logic.SecurityCheck(false, http.HandlerFunc(getUserRemoteAccessNetworks))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/rac/network/{network}/access_points", logic.SecurityCheck(false, http.HandlerFunc(getUserRemoteAccessNetworkGateways))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/rac/access_point/{access_point_id}/config", logic.SecurityCheck(false, http.HandlerFunc(getRemoteAccessGatewayConf))).Methods(http.MethodGet)
}
