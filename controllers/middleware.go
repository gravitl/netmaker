package controller

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
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
		r.Header.Set("NET_ID", params["network"])
		if strings.Contains(route, "hosts") || strings.Contains(route, "nodes") {
			r.Header.Set("TARGET_RSRC", models.HostRsrc.String())
		}
		if strings.Contains(route, "dns") {
			r.Header.Set("TARGET_RSRC", models.DnsRsrc.String())
		}
		if strings.Contains(route, "users") {
			r.Header.Set("TARGET_RSRC", models.UserRsrc.String())
		}
		if strings.Contains(route, "ingress") {
			r.Header.Set("TARGET_RSRC", models.RemoteAccessGwRsrc.String())
		}
		if strings.Contains(route, "createrelay") || strings.Contains(route, "deleterelay") {
			r.Header.Set("TARGET_RSRC", models.RelayRsrc.String())
		}

		if strings.Contains(route, "gateway") {
			r.Header.Set("TARGET_RSRC", models.EgressGwRsrc.String())
		}
		if strings.Contains(route, "networks") {
			r.Header.Set("TARGET_RSRC", models.NetworkRsrc.String())
		}
		if strings.Contains(route, "acls") {
			r.Header.Set("TARGET_RSRC", models.AclRsrc.String())
		}
		if strings.Contains(route, "extclients") {
			r.Header.Set("TARGET_RSRC", models.ExtClientsRsrc.String())
		}
		if strings.Contains(route, "enrollment-keys") {
			r.Header.Set("TARGET_RSRC", models.EnrollmentKeysRsrc.String())
		}
		if strings.Contains(route, "metrics") {
			r.Header.Set("TARGET_RSRC", models.MetricRsrc.String())
		}
		if keyID, ok := params["keyID"]; ok {
			r.Header.Set("TARGET_RSRC_ID", keyID)
		}
		if nodeID, ok := params["nodeid"]; ok && r.Header.Get("TARGET_RSRC") != models.ExtClientsRsrc.String() {
			r.Header.Set("TARGET_RSRC_ID", nodeID)
		}
		if strings.Contains(route, "failover") {
			r.Header.Set("TARGET_RSRC", models.FailOverRsrc.String())
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
			username, _ := url.QueryUnescape(r.URL.Query().Get("username"))
			if username != "" {
				r.Header.Set("TARGET_RSRC_ID", username)
			}
		}
		if r.Header.Get("NET_ID") == "" && (r.Header.Get("TARGET_RSRC_ID") == "" ||
			r.Header.Get("TARGET_RSRC") == models.EnrollmentKeysRsrc.String() ||
			r.Header.Get("TARGET_RSRC") == models.UserRsrc.String()) {
			r.Header.Set("IS_GLOBAL_ACCESS", "yes")
		}

		r.Header.Set("RSRC_TYPE", r.Header.Get("TARGET_RSRC"))
		handler.ServeHTTP(w, r)
	})
}
