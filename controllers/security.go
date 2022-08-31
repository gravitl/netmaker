package controller

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

const (
	master_uname     = "masteradministrator"
	unauthorized_msg = "unauthorized"
	unauthorized_err = models.Error(unauthorized_msg)
)

func securityCheck(reqAdmin bool, next http.Handler) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		var errorResponse = models.ErrorResponse{
			Code: http.StatusUnauthorized, Message: unauthorized_msg,
		}

		var params = mux.Vars(r)
		bearerToken := r.Header.Get("Authorization")
		// to have a custom DNS service adding entries
		// we should refactor this, but is for the special case of an external service to query the DNS api
		if strings.Contains(r.RequestURI, "/dns") && strings.ToUpper(r.Method) == "GET" && authenticateDNSToken(bearerToken) {
			// do dns stuff
			r.Header.Set("user", "nameserver")
			networks, _ := json.Marshal([]string{ALL_NETWORK_ACCESS})
			r.Header.Set("networks", string(networks))
			next.ServeHTTP(w, r)
			return
		}
		var networkName = params["networkname"]
		if len(networkName) == 0 {
			networkName = params["network"]
		}
		networks, username, err := SecurityCheck(reqAdmin, networkName, bearerToken)
		if err != nil {
			returnErrorResponse(w, r, errorResponse)
			return
		}
		networksJson, err := json.Marshal(&networks)
		if err != nil {
			returnErrorResponse(w, r, errorResponse)
			return
		}
		r.Header.Set("user", username)
		r.Header.Set("networks", string(networksJson))
		next.ServeHTTP(w, r)
	}
}

// SecurityCheck - checks token stuff
func SecurityCheck(reqAdmin bool, netname string, token string) ([]string, string, error) {
	var tokenSplit = strings.Split(token, " ")
	var authToken = ""
	userNetworks := []string{}

	if len(tokenSplit) < 2 {
		return userNetworks, "", unauthorized_err
	} else {
		authToken = tokenSplit[1]
	}
	//all endpoints here require master so not as complicated
	if authenticateMaster(authToken) {
		return []string{ALL_NETWORK_ACCESS}, master_uname, nil
	}
	username, networks, isadmin, err := logic.VerifyUserToken(authToken)
	if err != nil {
		return nil, username, unauthorized_err
	}
	if !isadmin && reqAdmin {
		return nil, username, unauthorized_err
	}
	userNetworks = networks
	if isadmin {
		return []string{ALL_NETWORK_ACCESS}, username, nil
	}
	// check network admin access
	if len(netname) > 0 && (!authenticateNetworkUser(netname, userNetworks) || len(userNetworks) == 0) {
		return nil, username, unauthorized_err
	}
	return userNetworks, username, nil
}

// Consider a more secure way of setting master key
func authenticateMaster(tokenString string) bool {
	return tokenString == servercfg.GetMasterKey() && servercfg.GetMasterKey() != ""
}

func authenticateNetworkUser(network string, userNetworks []string) bool {
	networkexists, err := functions.NetworkExists(network)
	if (err != nil && !database.IsEmptyRecord(err)) || !networkexists {
		return false
	}
	return logic.StringSliceContains(userNetworks, network)
}

//Consider a more secure way of setting master key
func authenticateDNSToken(tokenString string) bool {
	tokens := strings.Split(tokenString, " ")
	if len(tokens) < 2 {
		return false
	}
	return tokens[1] == servercfg.GetDNSKey()
}

func continueIfUserMatch(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var errorResponse = models.ErrorResponse{
			Code: http.StatusUnauthorized, Message: unauthorized_msg,
		}
		var params = mux.Vars(r)
		var requestedUser = params["username"]
		if requestedUser != r.Header.Get("user") {
			returnErrorResponse(w, r, errorResponse)
			return
		}
		next.ServeHTTP(w, r)
	}
}
