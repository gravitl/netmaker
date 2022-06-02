package controller

import (
	"crypto/ed25519"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/tls"
)

func serverHandlers(r *mux.Router) {
	// r.HandleFunc("/api/server/addnetwork/{network}", securityCheckServer(true, http.HandlerFunc(addNetwork))).Methods("POST")
	r.HandleFunc("/api/server/getconfig", securityCheckServer(false, http.HandlerFunc(getConfig))).Methods("GET")
	r.HandleFunc("/api/server/removenetwork/{network}", securityCheckServer(true, http.HandlerFunc(removeNetwork))).Methods("DELETE")
	r.HandleFunc("/api/server/register", authorize(true, false, "node", http.HandlerFunc(register))).Methods("POST")
	r.HandleFunc("/api/server/getserverinfo", authorize(true, false, "node", http.HandlerFunc(getServerInfo))).Methods("GET")
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

func getServerInfo(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params

	json.NewEncoder(w).Encode(servercfg.GetServerInfo())
	//w.WriteHeader(http.StatusOK)
}

func getConfig(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params

	scfg := servercfg.GetServerConfig()
	json.NewEncoder(w).Encode(scfg)
	//w.WriteHeader(http.StatusOK)
}

// register - registers a client with the server and return the CA and cert
func register(w http.ResponseWriter, r *http.Request) {
	logger.Log(2, "processing registration request")
	w.Header().Set("Content-Type", "application/json")
	//decode body
	var request config.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		logger.Log(0, "error decoding request", err.Error())
		errorResponse := models.ErrorResponse{
			Code: http.StatusBadRequest, Message: err.Error(),
		}
		returnErrorResponse(w, r, errorResponse)
		return
	}
	cert, ca, err := genCerts(&request.Key, &request.CommonName)
	if err != nil {
		logger.Log(0, "failed to generater certs ", err.Error())
		errorResponse := models.ErrorResponse{
			Code: http.StatusNotFound, Message: err.Error(),
		}
		returnErrorResponse(w, r, errorResponse)
		return
	}
	//x509.Certificate.PublicKey is an interface therefore json encoding/decoding result in a string value rather than a []byte
	//include the actual public key so the certificate can be properly reassembled on the other end.
	response := config.RegisterResponse{
		CA:         *ca,
		CAPubKey:   (ca.PublicKey).(ed25519.PublicKey),
		Cert:       *cert,
		CertPubKey: (cert.PublicKey).(ed25519.PublicKey),
		Broker:     servercfg.GetServer(),
		Port:       servercfg.GetMQPort(),
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// genCerts generates a client certificate and returns the certificate and root CA
func genCerts(clientKey *ed25519.PrivateKey, name *pkix.Name) (*x509.Certificate, *x509.Certificate, error) {
	ca, err := tls.ReadCert("/etc/netmaker/root.pem")
	if err != nil {
		logger.Log(2, "root ca not found ", err.Error())
		return nil, nil, fmt.Errorf("root ca not found %w", err)
	}
	key, err := tls.ReadKey("/etc/netmaker/root.key")
	if err != nil {
		logger.Log(2, "root key not found ", err.Error())
		return nil, nil, fmt.Errorf("root key not found %w", err)
	}
	csr, err := tls.NewCSR(*clientKey, *name)
	if err != nil {
		logger.Log(2, "failed to generate client certificate requests", err.Error())
		return nil, nil, fmt.Errorf("client certification request generation failed %w", err)
	}
	cert, err := tls.NewEndEntityCert(*key, csr, ca, tls.CERTIFICATE_VALIDITY)
	if err != nil {
		logger.Log(2, "unable to generate client certificate", err.Error())
		return nil, nil, fmt.Errorf("client certification generation failed %w", err)
	}
	return cert, ca, nil
}
