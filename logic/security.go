package logic

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

const (
	// ALL_NETWORK_ACCESS - represents all networks
	ALL_NETWORK_ACCESS = "THIS_USER_HAS_ALL"

	master_uname     = "masteradministrator"
	Forbidden_Msg    = "forbidden"
	Forbidden_Err    = models.Error(Forbidden_Msg)
	Unauthorized_Msg = "unauthorized"
	Unauthorized_Err = models.Error(Unauthorized_Msg)
)

// SecurityCheck - Check if user has appropriate permissions
func SecurityCheck(reqAdmin bool, next http.Handler) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		var errorResponse = models.ErrorResponse{
			Code: http.StatusForbidden, Message: Forbidden_Msg,
		}
		r.Header.Set("ismaster", "no")

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
		networks, username, err := UserPermissions(reqAdmin, networkName, bearerToken)
		if err != nil {
			ReturnErrorResponse(w, r, errorResponse)
			return
		}
		// detect masteradmin
		if len(networks) > 0 && networks[0] == ALL_NETWORK_ACCESS {
			r.Header.Set("ismaster", "yes")
		}
		networksJson, err := json.Marshal(&networks)
		if err != nil {
			ReturnErrorResponse(w, r, errorResponse)
			return
		}
		r.Header.Set("user", username)
		r.Header.Set("networks", string(networksJson))
		next.ServeHTTP(w, r)
	}
}

// UserPermissions - checks token stuff
func UserPermissions(reqAdmin bool, netname string, token string) ([]string, string, error) {
	var tokenSplit = strings.Split(token, " ")
	var authToken = ""
	userNetworks := []string{}

	if len(tokenSplit) < 2 {
		return userNetworks, "", Unauthorized_Err
	} else {
		authToken = tokenSplit[1]
	}
	//all endpoints here require master so not as complicated
	if authenticateMaster(authToken) {
		// TODO log in as an actual admin user
		return []string{ALL_NETWORK_ACCESS}, master_uname, nil
	}
	username, networks, isadmin, err := VerifyUserToken(authToken)
	if err != nil {
		return nil, username, Unauthorized_Err
	}
	if !isadmin && reqAdmin {
		return nil, username, Forbidden_Err
	}
	userNetworks = networks
	if isadmin {
		return []string{ALL_NETWORK_ACCESS}, username, nil
	}
	// check network admin access
	if len(netname) > 0 && (len(userNetworks) == 0 || !authenticateNetworkUser(netname, userNetworks)) {
		return nil, username, Forbidden_Err
	}
	if isEE && len(netname) > 0 {
		return nil, "", Forbidden_Err
	}
	return userNetworks, username, nil
}

// Consider a more secure way of setting master key
func authenticateMaster(tokenString string) bool {
	return tokenString == servercfg.GetMasterKey() && servercfg.GetMasterKey() != ""
}

func authenticateNetworkUser(network string, userNetworks []string) bool {
	networkexists, err := NetworkExists(network)
	if (err != nil && !database.IsEmptyRecord(err)) || !networkexists {
		return false
	}
	return StringSliceContains(userNetworks, network)
}

// Consider a more secure way of setting master key
func authenticateDNSToken(tokenString string) bool {
	tokens := strings.Split(tokenString, " ")
	if len(tokens) < 2 {
		return false
	}
	return len(servercfg.GetDNSKey()) > 0 && tokens[1] == servercfg.GetDNSKey()
}

func ContinueIfUserMatch(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var errorResponse = models.ErrorResponse{
			Code: http.StatusForbidden, Message: Forbidden_Msg,
		}
		var params = mux.Vars(r)
		var requestedUser = params["username"]
		if requestedUser != r.Header.Get("user") {
			ReturnErrorResponse(w, r, errorResponse)
			return
		}
		next.ServeHTTP(w, r)
	}
}
