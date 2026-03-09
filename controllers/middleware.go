package controller

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/schema"
)

func userMiddleWare(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var params = mux.Vars(r)
		route, err := mux.CurrentRoute(r).GetPathTemplate()
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
		if r.Method == http.MethodPost && route == "/api/extclients/{network}/{nodeid}" {
			node, err := logic.GetNodeByID(params["nodeid"])
			if err == nil {
				params["network"] = node.Network
			}
		}
		r.Header.Set("IS_GLOBAL_ACCESS", "no")
		r.Header.Set("TARGET_RSRC", "")
		r.Header.Set("RSRC_TYPE", "")
		r.Header.Set("TARGET_RSRC_ID", "")
		r.Header.Set("RAC", "")
		r.Header.Set("NET_ID", params["network"])
		if r.URL.Query().Get("network") != "" {
			r.Header.Set("NET_ID", r.URL.Query().Get("network"))
		}
		if strings.Contains(route, "hosts") || strings.Contains(route, "nodes") {
			r.Header.Set("TARGET_RSRC", schema.HostRsrc.String())
		}
		if strings.Contains(route, "dns") {
			r.Header.Set("TARGET_RSRC", schema.DnsRsrc.String())
		}
		if strings.Contains(route, "rac") {
			r.Header.Set("RAC", "true")
		}
		if strings.Contains(route, "users") {
			r.Header.Set("TARGET_RSRC", schema.UserRsrc.String())
		}
		if strings.Contains(route, "ingress") {
			r.Header.Set("TARGET_RSRC", schema.RemoteAccessGwRsrc.String())
		}
		if strings.Contains(route, "createrelay") || strings.Contains(route, "deleterelay") {
			r.Header.Set("TARGET_RSRC", schema.RelayRsrc.String())
		}
		if strings.Contains(route, "gateway") {
			r.Header.Set("TARGET_RSRC", schema.GatewayRsrc.String())
		}

		if strings.Contains(route, "egress") {
			r.Header.Set("TARGET_RSRC", schema.EgressGwRsrc.String())
		}
		if strings.Contains(route, "networks") {
			r.Header.Set("TARGET_RSRC", schema.NetworkRsrc.String())
		}
		// check 'graph' after 'networks', otherwise the
		// header will be overwritten.
		if strings.Contains(route, "graph") {
			r.Header.Set("TARGET_RSRC", schema.HostRsrc.String())
		}
		if strings.Contains(route, "acls") {
			r.Header.Set("TARGET_RSRC", schema.AclRsrc.String())
		}
		if strings.Contains(route, "tags") {
			r.Header.Set("TARGET_RSRC", schema.TagRsrc.String())
		}
		if strings.Contains(route, "extclients") || strings.Contains(route, "client_conf") {
			r.Header.Set("TARGET_RSRC", schema.ExtClientsRsrc.String())
		}
		if strings.Contains(route, "enrollment-keys") {
			r.Header.Set("TARGET_RSRC", schema.EnrollmentKeysRsrc.String())
		}
		if strings.Contains(route, "posture_check") {
			r.Header.Set("TARGET_RSRC", schema.PostureCheckRsrc.String())
		}
		if strings.Contains(route, "activity") {
			r.Header.Set("TARGET_RSRC", schema.UserActivityRsrc.String())
		}
		if strings.Contains(route, "nameserver") {
			r.Header.Set("TARGET_RSRC", schema.NameserverRsrc.String())
		}
		if strings.Contains(route, "jit") {
			r.Header.Set("TARGET_RSRC", schema.JitAdminRsrc.String())
		}
		if strings.Contains(route, "jit_user") {
			r.Header.Set("TARGET_RSRC", schema.JitUserRsrc.String())
		}
		if strings.Contains(route, "metrics") {
			r.Header.Set("TARGET_RSRC", schema.MetricRsrc.String())
		}
		if strings.Contains(route, "flows") {
			r.Header.Set("TARGET_RSRC", schema.TrafficFlow.String())
		}
		if keyID, ok := params["keyID"]; ok {
			r.Header.Set("TARGET_RSRC_ID", keyID)
		}
		if nodeID, ok := params["nodeid"]; ok && r.Header.Get("TARGET_RSRC") != schema.ExtClientsRsrc.String() {
			r.Header.Set("TARGET_RSRC_ID", nodeID)
		}
		if strings.Contains(route, "failover") {
			r.Header.Set("TARGET_RSRC", schema.FailOverRsrc.String())
			nodeID := r.Header.Get("TARGET_RSRC_ID")
			node, _ := logic.GetNodeByID(nodeID)
			r.Header.Set("NET_ID", node.Network)

		}
		if hostID, ok := params["hostid"]; ok {
			r.Header.Set("TARGET_RSRC_ID", hostID)
		}
		if clientID, ok := params["clientid"]; ok {
			r.Header.Set("TARGET_RSRC_ID", clientID)
		}
		if netID, ok := params["networkname"]; ok {
			if !strings.Contains(route, "acls") {
				r.Header.Set("TARGET_RSRC_ID", netID)
			}
			r.Header.Set("NET_ID", params["networkname"])
		}

		if userID, ok := params["username"]; ok {
			r.Header.Set("TARGET_RSRC_ID", userID)
		} else {
			username := r.URL.Query().Get("username")
			if username != "" {
				r.Header.Set("TARGET_RSRC_ID", username)
			}
		}
		if r.Header.Get("NET_ID") == "" && (r.Header.Get("TARGET_RSRC_ID") == "" ||
			r.Header.Get("TARGET_RSRC") == schema.EnrollmentKeysRsrc.String() ||
			r.Header.Get("TARGET_RSRC") == schema.UserRsrc.String()) ||
			(r.Header.Get("TARGET_RSRC") == schema.UserActivityRsrc.String() && route != "/api/v1/network/activity") ||
			r.Header.Get("TARGET_RSRC") == schema.TrafficFlow.String() {
			r.Header.Set("IS_GLOBAL_ACCESS", "yes")
		}
		r.Header.Set("RSRC_TYPE", r.Header.Get("TARGET_RSRC"))
		handler.ServeHTTP(w, r)
	})
}
