package controller

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/servercfg"
)

func serverHandlers(r *mux.Router) {
	// r.HandleFunc("/api/server/addnetwork/{network}", securityCheckServer(true, http.HandlerFunc(addNetwork))).Methods(http.MethodPost)
	r.HandleFunc("/api/server/health", http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		resp.WriteHeader(http.StatusOK)
		resp.Write([]byte("Server is up and running!!"))
	}))
	r.HandleFunc("/api/server/getconfig", allowUsers(http.HandlerFunc(getConfig))).Methods(http.MethodGet)
	r.HandleFunc("/api/server/getserverinfo", authorize(true, false, "node", http.HandlerFunc(getServerInfo))).Methods(http.MethodGet)
	r.HandleFunc("/api/server/status", http.HandlerFunc(getStatus)).Methods(http.MethodGet)
	r.HandleFunc("/api/server/usage", authorize(true, false, "user", http.HandlerFunc(getUsage))).Methods(http.MethodGet)
}
func getUsage(w http.ResponseWriter, r *http.Request) {
	type usage struct {
		Hosts    int `json:"hosts"`
		Clients  int `json:"clients"`
		Networks int `json:"networks"`
		Users    int `json:"users"`
	}
	var serverUsage usage
	hosts, err := logic.GetAllHosts()
	if err == nil {
		serverUsage.Hosts = len(hosts)
	}
	clients, err := logic.GetAllExtClients()
	if err == nil {
		serverUsage.Clients = len(clients)
	}
	users, err := logic.GetUsers()
	if err == nil {
		serverUsage.Users = len(users)
	}
	networks, err := logic.GetNetworks()
	if err == nil {
		serverUsage.Networks = len(networks)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&serverUsage)

}

// swagger:route GET /api/server/status server getStatus
//
// Get the server configuration.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: serverConfigResponse
func getStatus(w http.ResponseWriter, r *http.Request) {
	// TODO
	// - check health of broker
	type status struct {
		DB     bool `json:"db_connected"`
		Broker bool `json:"broker_connected"`
		Usage  struct {
			Hosts    int `json:"hosts"`
			Clients  int `json:"clients"`
			Networks int `json:"networks"`
			Users    int `json:"users"`
		} `json:"usage"`
	}

	currentServerStatus := status{
		DB:     database.IsConnected(),
		Broker: mq.IsConnected(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&currentServerStatus)
}

// allowUsers - allow all authenticated (valid) users - only used by getConfig, may be able to remove during refactor
func allowUsers(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var errorResponse = models.ErrorResponse{
			Code: http.StatusInternalServerError, Message: logic.Unauthorized_Msg,
		}
		bearerToken := r.Header.Get("Authorization")
		var tokenSplit = strings.Split(bearerToken, " ")
		var authToken = ""
		if len(tokenSplit) < 2 {
			logic.ReturnErrorResponse(w, r, errorResponse)
			return
		} else {
			authToken = tokenSplit[1]
		}
		user, _, _, err := logic.VerifyUserToken(authToken)
		if err != nil || user == "" {
			logic.ReturnErrorResponse(w, r, errorResponse)
			return
		}
		next.ServeHTTP(w, r)
	}
}

// swagger:route GET /api/server/getserverinfo server getServerInfo
//
// Get the server configuration.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: serverConfigResponse
func getServerInfo(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params

	json.NewEncoder(w).Encode(servercfg.GetServerInfo())
	//w.WriteHeader(http.StatusOK)
}

// swagger:route GET /api/server/getconfig server getConfig
//
// Get the server configuration.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: serverConfigResponse
func getConfig(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params

	scfg := servercfg.GetServerConfig()
	scfg.IsEE = "no"
	if servercfg.Is_EE {
		scfg.IsEE = "yes"
	}
	json.NewEncoder(w).Encode(scfg)
	//w.WriteHeader(http.StatusOK)
}
