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
		}

		err, networks, username := SecurityCheck(reqAdmin, params["networkname"], bearerToken)
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
func SecurityCheck(reqAdmin bool, netname string, token string) (error, []string, string) {

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
			return errors.New("error verifying user token"), nil, username
		}
		if !isadmin && reqAdmin {
			return errors.New("you are unauthorized to access this endpoint"), nil, username
		}
		userNetworks = networks
		if isadmin {
			userNetworks = []string{ALL_NETWORK_ACCESS}
		} else {
			networkexists, err := functions.NetworkExists(netname)
			if err != nil && !database.IsEmptyRecord(err) {
				return err, nil, ""
			}
			if netname != "" && !networkexists {
				return errors.New("this network does not exist"), nil, ""
			}
		}
	} else if isMasterAuthenticated {
		userNetworks = []string{ALL_NETWORK_ACCESS}
	}
	if len(userNetworks) == 0 {
		userNetworks = append(userNetworks, NO_NETWORKS_PRESENT)
	}
	return nil, userNetworks, username
}

//Consider a more secure way of setting master key
func authenticateMaster(tokenString string) bool {
	return tokenString == servercfg.GetMasterKey()
}

//Consider a more secure way of setting master key
func authenticateDNSToken(tokenString string) bool {
	tokens := strings.Split(tokenString, " ")
	if len(tokens) < 2 {
		return false
	}
	return tokens[1] == servercfg.GetDNSKey()
}

// ValidateUserToken - self explained
func ValidateUserToken(token string, user string, adminonly bool) error {
	var tokenSplit = strings.Split(token, " ")
	//I put this in in case the user doesn't put in a token at all (in which case it's empty)
	//There's probably a smarter way of handling this.
	var authToken = "928rt238tghgwe@TY@$Y@#WQAEGB2FC#@HG#@$Hddd"

	if len(tokenSplit) > 1 {
		authToken = tokenSplit[1]
	} else {
		return errors.New("Missing Auth Token.")
	}

	username, _, isadmin, err := logic.VerifyUserToken(authToken)
	if err != nil {
		return errors.New("Error Verifying Auth Token")
	}
	isAuthorized := false
	if adminonly {
		isAuthorized = isadmin
	} else {
		isAuthorized = username == user || isadmin
	}
	if !isAuthorized {
		return errors.New("You are unauthorized to access this endpoint.")
	}

	return nil
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
