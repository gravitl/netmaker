package auth

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic/pro/netcache"
	"github.com/gravitl/netmaker/scope"
)

var (
	redirectUrl string
)

// HandleHostSSOCallback handles the callback from the sso endpoint for host registration.
// Registering a host via SSO is no longer supported, so the request is rejected.
// Listens in /oidc/callback.
func HandleHostSSOCallback(w http.ResponseWriter, r *http.Request) {
	logger.Log(0, "host registration via SSO attempted, but it is no longer supported")
	w.WriteHeader(http.StatusGone)
	_, _ = w.Write([]byte("registering a host via SSO is no longer supported; use an enrollment token: netclient join -t <token>"))
}

func setNetcache(ncache *netcache.CValue, state string) error {
	if ncache == nil {
		return fmt.Errorf("cache miss")
	}
	var err error
	if err = netcache.Set(state, ncache); err != nil {
		logger.Log(0, "machine failed to complete join on network,", ncache.Network, "-", err.Error())
	}
	return err
}

func returnErrTemplate(uname, message, state string, ncache *netcache.CValue) []byte {
	var response bytes.Buffer
	if ncache != nil {
		ncache.Pass = message
	}
	err := ssoErrCallbackTemplate.Execute(&response, ssoCallbackTemplateConfig{
		User: uname,
		Verb: message,
	})
	if err != nil {
		return []byte(err.Error())
	}
	err = setNetcache(ncache, state)
	if err != nil {
		return []byte(err.Error())
	}
	return response.Bytes()
}

// RegisterHostSSO redirects to the IDP for authentication
// Puts machine key in cache so the callback can retrieve it using the oidc state param
// Listens in /oidc/register/:regKey.
func RegisterHostSSO(w http.ResponseWriter, r *http.Request) {
	machineKeyStr := mux.Vars(r)["regKey"]
	if machineKeyStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid login attempt: no key"))
		return
	}

	state, err := netcache.Get(machineKeyStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid login attempt: no state"))
		return
	}

	ctx := scope.WithContext(r.Context(), state.Scope, state.ScopeID)
	p, ok := Registry().FromContext(ctx)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid login attempt: no auth provider"))
		return
	}

	http.Redirect(w, r, p.Config().AuthCodeURL(machineKeyStr), http.StatusSeeOther)
}
