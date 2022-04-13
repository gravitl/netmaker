package controller

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/tls"
)

func serverHandlers(r *mux.Router) {
	// r.HandleFunc("/api/server/addnetwork/{network}", securityCheckServer(true, http.HandlerFunc(addNetwork))).Methods("POST")
	r.HandleFunc("/api/server/getconfig", securityCheckServer(false, http.HandlerFunc(getConfig))).Methods("GET")
	r.HandleFunc("/api/server/removenetwork/{network}", securityCheckServer(true, http.HandlerFunc(removeNetwork))).Methods("DELETE")
	r.HandleFunc("/api/server/register/", http.HandlerFunc(register)).Methods("POST")
}

//Security check is middleware for every function and just checks to make sure that its the master calling
//Only admin should have access to all these network-level actions
//or maybe some Users once implemented
func securityCheckServer(adminonly bool, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var errorResponse = models.ErrorResponse{
			Code: http.StatusInternalServerError, Message: "W1R3: It's not you it's me.",
		}

		bearerToken := r.Header.Get("Authorization")

		var tokenSplit = strings.Split(bearerToken, " ")
		var authToken = ""
		if len(tokenSplit) < 2 {
			errorResponse = models.ErrorResponse{
				Code: http.StatusUnauthorized, Message: "W1R3: You are unauthorized to access this endpoint.",
			}
			returnErrorResponse(w, r, errorResponse)
			return
		} else {
			authToken = tokenSplit[1]
		}
		//all endpoints here require master so not as complicated
		//still might not be a good  way of doing this
		user, _, isadmin, err := logic.VerifyUserToken(authToken)
		errorResponse = models.ErrorResponse{
			Code: http.StatusUnauthorized, Message: "W1R3: You are unauthorized to access this endpoint.",
		}
		if !adminonly && (err != nil || user == "") {
			returnErrorResponse(w, r, errorResponse)
			return
		}
		if adminonly && !isadmin && !authenticateMaster(authToken) {
			returnErrorResponse(w, r, errorResponse)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func removeNetwork(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params
	var params = mux.Vars(r)

	err := logic.DeleteNetwork(params["network"])
	if err != nil {
		json.NewEncoder(w).Encode("Could not remove server from network " + params["network"])
		return
	}

	json.NewEncoder(w).Encode("Server removed from network " + params["network"])
}

func getConfig(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params

	scfg := servercfg.GetServerConfig()
	json.NewEncoder(w).Encode(scfg)
	//w.WriteHeader(http.StatusOK)
}

// func addNetwork(w http.ResponseWriter, r *http.Request) {
// 	// Set header
// 	w.Header().Set("Content-Type", "application/json")

// 	// get params
// 	var params = mux.Vars(r)
// 	var networkName = params["network"]
// 	var networkSettings, err := logic.GetNetwork(netwnetworkName)

// 	success, err := serverctl.AddNetwork(params["network"])

// 	if err != nil || !success {
// 		json.NewEncoder(w).Encode("Could not add server to network " + params["network"])
// 		return
// 	}

// 	json.NewEncoder(w).Encode("Server added to network " + params["network"])
// }

// register - registers a client with the server and return the CA cert
func register(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	token := r.Header.Get("Authorization")
	found := false
	networks, err := logic.GetNetworks()
	if err != nil {
		errorResponse := models.ErrorResponse{
			Code: http.StatusNotFound, Message: "no networks",
		}
		returnErrorResponse(w, r, errorResponse)
	}
	for _, network := range networks {
		for _, key := range network.AccessKeys {
			if key.AccessString == token {
				found = true
				break
			}
		}
	}
	if !found {
		errorResponse := models.ErrorResponse{
			Code: http.StatusUnauthorized, Message: "You are unauthorized to access this endpoint.",
		}
		returnErrorResponse(w, r, errorResponse)
		return
	}
	ca, err := tls.ReadCert("/etc/netmaker/root.pem")
	if err != nil {
		errorResponse := models.ErrorResponse{
			Code: http.StatusNotFound, Message: "root ca not found",
		}
		returnErrorResponse(w, r, errorResponse)
		return
		//return err
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(*ca)
}
