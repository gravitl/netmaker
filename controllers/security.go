package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

func securityCheck(reqAdmin bool, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var errorResponse = models.ErrorResponse{
			Code: http.StatusUnauthorized, Message: "W1R3: It's not you it's me.",
		}

		var params = mux.Vars(r)
		bearerToken := r.Header.Get("Authorization")
		if strings.Contains(r.RequestURI, "/dns") && strings.ToUpper(r.Method) == "GET" && authenticateDNSToken(bearerToken) {
			// do dns stuff
			r.Header.Set("user", "nameserver")
			networks, _ := json.Marshal([]string{ALL_NETWORK_ACCESS})
			r.Header.Set("networks", string(networks))
			next.ServeHTTP(w, r)
			return
		}

		networks, username, err := SecurityCheck(reqAdmin, params["networkname"], bearerToken)
		if err != nil {
			if strings.Contains(err.Error(), "does not exist") {
				errorResponse.Code = http.StatusNotFound
			}
			errorResponse.Message = err.Error()
			returnErrorResponse(w, r, errorResponse)
			return
		}
		networksJson, err := json.Marshal(&networks)
		if err != nil {
			errorResponse.Message = err.Error()
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

	var hasBearer = true
	var tokenSplit = strings.Split(token, " ")
	var authToken = ""

	if len(tokenSplit) < 2 {
		hasBearer = false
	} else {
		authToken = tokenSplit[1]
	}
	userNetworks := []string{}
	//all endpoints here require master so not as complicated
	isMasterAuthenticated := authenticateMaster(authToken)
	username := ""
	if !hasBearer || !isMasterAuthenticated {
		userName, networks, isadmin, err := logic.VerifyUserToken(authToken)
		username = userName
		if err != nil {
			return nil, username, errors.New("error verifying user token")
		}
		if !isadmin && reqAdmin {
			return nil, username, errors.New("you are unauthorized to access this endpoint")
		}
		userNetworks = networks
		if isadmin {
			userNetworks = []string{ALL_NETWORK_ACCESS}
		} else {
			networkexists, err := functions.NetworkExists(netname)
			if err != nil && !database.IsEmptyRecord(err) {
				return nil, "", err
			}
			if netname != "" && !networkexists {
				return nil, "", errors.New("this network does not exist")
			}
		}
	} else if isMasterAuthenticated {
		userNetworks = []string{ALL_NETWORK_ACCESS}
	}
	if len(userNetworks) == 0 {
		userNetworks = append(userNetworks, NO_NETWORKS_PRESENT)
	}
	return userNetworks, username, nil
}

// Consider a more secure way of setting master key
func authenticateMaster(tokenString string) bool {
	return tokenString == servercfg.GetMasterKey() && servercfg.GetMasterKey() != ""
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
			Code: http.StatusUnauthorized, Message: "W1R3: This doesn't look like you.",
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
