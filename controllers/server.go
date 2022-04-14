package controller

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/tls"
	"github.com/kr/pretty"
)

func serverHandlers(r *mux.Router) {
	// r.HandleFunc("/api/server/addnetwork/{network}", securityCheckServer(true, http.HandlerFunc(addNetwork))).Methods("POST")
	r.HandleFunc("/api/server/getconfig", securityCheckServer(false, http.HandlerFunc(getConfig))).Methods("GET")
	r.HandleFunc("/api/server/removenetwork/{network}", securityCheckServer(true, http.HandlerFunc(removeNetwork))).Methods("DELETE")
	r.HandleFunc("/api/server/register", http.HandlerFunc(register)).Methods("POST")
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
	bearerToken := r.Header.Get("Authorization")
	var tokenSplit = strings.Split(bearerToken, " ")
	var token = ""
	if len(tokenSplit) < 2 {
		errorResponse := models.ErrorResponse{
			Code: http.StatusUnauthorized, Message: "W1R3: You are unauthorized to access this endpoint.",
		}
		returnErrorResponse(w, r, errorResponse)
		return
	} else {
		token = tokenSplit[1]
	}
	//decode body
	var request config.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		logger.Log(3, "error decoding request")
		errorResponse := models.ErrorResponse{
			Code: http.StatusBadRequest, Message: "invalid request",
		}
		returnErrorResponse(w, r, errorResponse)
	}
	found := false
	networks, err := logic.GetNetworks()
	if err != nil {
		logger.Log(3, "no networks")
		errorResponse := models.ErrorResponse{
			Code: http.StatusNotFound, Message: "no networks",
		}
		returnErrorResponse(w, r, errorResponse)
	}
	for _, network := range networks {
		for _, key := range network.AccessKeys {
			if key.Value == token {
				found = true
				break
			}
		}
	}
	if !found {
		logger.Log(2, "valid access key not found")
		errorResponse := models.ErrorResponse{
			Code: http.StatusUnauthorized, Message: "You are unauthorized to access this endpoint.",
		}
		returnErrorResponse(w, r, errorResponse)
		return
	}
	ca, err := tls.ReadCert("/etc/netmaker/root.pem")
	if err != nil {
		logger.Log(2, "root ca not found ", err.Error())
		errorResponse := models.ErrorResponse{
			Code: http.StatusNotFound, Message: "root ca not found",
		}
		returnErrorResponse(w, r, errorResponse)
		return
	}
	key, err := tls.ReadKey("/etc/netmaker/root.key")
	if err != nil {
		logger.Log(2, "root key not found ", err.Error())
		errorResponse := models.ErrorResponse{
			Code: http.StatusNotFound, Message: "root key not found",
		}
		returnErrorResponse(w, r, errorResponse)
		return
	}
	pretty.Println(&request.CSR.PublicKey)
	pretty.Println(request.CSR.RawSubjectPublicKeyInfo)
	cert, err := tls.NewEndEntityCert(*key, &request.CSR, ca, tls.CERTIFICATE_VALIDITY)
	if err != nil {
		logger.Log(2, "unable to generate client certificate", err.Error())
		errorResponse := models.ErrorResponse{
			Code: http.StatusInternalServerError, Message: err.Error(),
		}
		returnErrorResponse(w, r, errorResponse)
		return
	}
	response := config.RegisterResponse{
		CA:   *ca,
		Cert: *cert,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
