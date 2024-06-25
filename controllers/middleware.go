package controller

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/models"
)

func userMiddleWare(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var params = mux.Vars(r)
		r.Header.Set("NET_ID", params["network"])
		if strings.Contains(r.URL.Path, "hosts") || strings.Contains(r.URL.Path, "nodes") {
			r.Header.Set("TARGET_RSRC", models.HostRsrc.String())
			r.Header.Set("RSRC_TYPE", models.HostRsrc.String())
		}
		if strings.Contains(r.URL.Path, "dns") {
			r.Header.Set("RSRC_TYPE", models.DnsRsrc.String())
			r.Header.Set("TARGET_RSRC", models.DnsRsrc.String())
		}
		if strings.Contains(r.URL.Path, "users") {
			r.Header.Set("RSRC_TYPE", models.UserRsrc.String())
			r.Header.Set("TARGET_RSRC", models.UserRsrc.String())
		}
		if strings.Contains(r.URL.Path, "ingress") {
			r.Header.Set("TARGET_RSRC", models.RemoteAccessGwRsrc.String())
		}
		if strings.Contains(r.URL.Path, "gateway") {
			r.Header.Set("TARGET_RSRC", models.EgressGwRsrc.String())
		}
		if strings.Contains(r.URL.Path, "networks") {
			r.Header.Set("TARGET_RSRC", models.NetworkRsrc.String())
			r.Header.Set("RSRC_TYPE", models.NetworkRsrc.String())
		}
		if strings.Contains(r.URL.Path, "acls") {
			r.Header.Set("TARGET_RSRC", models.AclRsrc.String())
			r.Header.Set("RSRC_TYPE", models.NetworkRsrc.String())
		}
		if strings.Contains(r.URL.Path, "extclients") {
			r.Header.Set("TARGET_RSRC", models.ExtClientsRsrc.String())
			r.Header.Set("RSRC_TYPE", models.ExtClientsRsrc.String())
		}
		if strings.Contains(r.URL.Path, "enrollment-keys") {
			r.Header.Set("TARGET_RSRC", models.EnrollmentKeysRsrc.String())
			r.Header.Set("RSRC_TYPE", models.EnrollmentKeysRsrc.String())
		}
		if keyID, ok := params["keyID"]; ok {
			r.Header.Set("TARGET_RSRC_ID", keyID)
		}
		if nodeID, ok := params["nodeid"]; ok {
			r.Header.Set("TARGET_RSRC_ID", nodeID)
		}
		if hostID, ok := params["hostid"]; ok {
			r.Header.Set("TARGET_RSRC_ID", hostID)
		}
		if clientID, ok := params["clientid"]; ok {
			r.Header.Set("TARGET_RSRC_ID", clientID)
		}
		if netID, ok := params["networkname"]; ok {
			if !strings.Contains(r.URL.Path, "acls") {
				r.Header.Set("TARGET_RSRC_ID", netID)
			}
			r.Header.Set("NET_ID", params["networkname"])
		}

		if userID, ok := params["username"]; ok {
			r.Header.Set("TARGET_RSRC_ID", userID)
		}
		if r.Header.Get("TARGET_RSRC_ID") == "" ||
			r.Header.Get("TARGET_RSRC") == models.EnrollmentKeysRsrc.String() ||
			r.Header.Get("TARGET_RSRC") == models.UserRsrc.String() {
			r.Header.Set("IS_GLOBAL_ACCESS", "yes")
		}
		handler.ServeHTTP(w, r)
	})
}
