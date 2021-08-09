package controller

import (
    "github.com/gravitl/netmaker/models"
    "github.com/gravitl/netmaker/functions"
    "github.com/gravitl/netmaker/serverctl"
    "github.com/gravitl/netmaker/servercfg"
    "encoding/json"
    "strings"
    "net/http"
    "github.com/gorilla/mux"
)

func serverHandlers(r *mux.Router) {
    r.HandleFunc("/api/server/addnetwork/{network}", securityCheckServer(http.HandlerFunc(addNetwork))).Methods("POST")
    r.HandleFunc("/api/server/getconfig", securityCheckServer(http.HandlerFunc(getConfig))).Methods("GET")
    r.HandleFunc("/api/server/getwgconfig", securityCheckServer(http.HandlerFunc(getWGConfig))).Methods("GET")
    r.HandleFunc("/api/server/removenetwork/{network}", securityCheckServer(http.HandlerFunc(removeNetwork))).Methods("DELETE")
}

//Security check is middleware for every function and just checks to make sure that its the master calling
//Only admin should have access to all these network-level actions
//or maybe some Users once implemented
func securityCheckServer(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var errorResponse = models.ErrorResponse{
			Code: http.StatusInternalServerError, Message: "W1R3: It's not you it's me.",
		}

		bearerToken := r.Header.Get("Authorization")

		var tokenSplit = strings.Split(bearerToken, " ")
		var  authToken = ""
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
                _, _, isadmin, _ := functions.VerifyUserToken(authToken)

		if !isadmin && !authenticateMasterServer(authToken) {
				errorResponse = models.ErrorResponse{
					Code: http.StatusUnauthorized, Message: "W1R3: You are unauthorized to access this endpoint.",
				}
				returnErrorResponse(w, r, errorResponse)
				return
		}
		next.ServeHTTP(w, r)
	}
}
//Consider a more secure way of setting master key
func authenticateMasterServer(tokenString string) bool {
    if tokenString == servercfg.GetMasterKey() {
        return true
    }
    return false
}

func removeNetwork(w http.ResponseWriter, r *http.Request) {
        // Set header
        w.Header().Set("Content-Type", "application/json")

        // get params
        var params = mux.Vars(r)

        success, err := serverctl.RemoveNetwork(params["network"])

        if err != nil || !success {
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
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(scfg)
}

func getWGConfig(w http.ResponseWriter, r *http.Request) {
        // Set header
        w.Header().Set("Content-Type", "application/json")

        // get params

        wgcfg := servercfg.GetWGConfig()
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(wgcfg)
}

/*
func getMongoConfig(w http.ResponseWriter, r *http.Request) {
        // Set header
        w.Header().Set("Content-Type", "application/json")

        // get params

        mcfg := servercfg.GetMongoConfig()
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(mcfg)
}
*/

func addNetwork(w http.ResponseWriter, r *http.Request) {
        // Set header
        w.Header().Set("Content-Type", "application/json")

        // get params
        var params = mux.Vars(r)

        success, err := serverctl.AddNetwork(params["network"])

        if err != nil || !success {
                json.NewEncoder(w).Encode("Could not add server to network " + params["network"])
                return
        }

        json.NewEncoder(w).Encode("Server added to network " + params["network"])
}
