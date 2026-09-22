package controllers

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/middleware"
	"github.com/gravitl/netmaker/scope"
)

func RacHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/rac/networks", middleware.Scope(scope.TenantScope, logic.SecurityCheck(false, http.HandlerFunc(getUserRemoteAccessNetworks)))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/rac/network/{network}/access_points", middleware.Scope(scope.TenantScope, logic.SecurityCheck(false, http.HandlerFunc(getUserRemoteAccessNetworkGateways)))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/rac/access_point/{access_point_id}/config", middleware.Scope(scope.TenantScope, logic.SecurityCheck(false, http.HandlerFunc(getRemoteAccessGatewayConf)))).Methods(http.MethodGet)
}
