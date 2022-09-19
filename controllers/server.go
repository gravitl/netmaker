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
	"github.com/gravitl/netmaker/serverctl"
	"github.com/gravitl/netmaker/tls"
)

func serverHandlers(r *mux.Router) {
	// r.HandleFunc("/api/server/addnetwork/{network}", securityCheckServer(true, http.HandlerFunc(addNetwork))).Methods("POST")
	r.HandleFunc("/api/server/getconfig", allowUsers(http.HandlerFunc(getConfig))).Methods("GET")
	r.HandleFunc("/api/server/register", authorize(true, false, "node", http.HandlerFunc(register))).Methods("POST")
	r.HandleFunc("/api/server/getserverinfo", authorize(true, false, "node", http.HandlerFunc(getServerInfo))).Methods("GET")
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
//		Schemes: https
//
// 		Security:
//   		oauth
//
//		Responses:
//			200: serverConfigResponse
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
//		Schemes: https
//
// 		Security:
//   		oauth
//
//		Responses:
//			200: serverConfigResponse
func getConfig(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params

	scfg := servercfg.GetServerConfig()
	scfg.IsEE = "no"
	if logic.Is_EE {
		scfg.IsEE = "yes"
	}
	json.NewEncoder(w).Encode(scfg)
	//w.WriteHeader(http.StatusOK)
}

// swagger:route POST /api/server/register server register
//
// Registers a client with the server and return the Certificate Authority and certificate.
//
//		Schemes: https
//
// 		Security:
//   		oauth
//
//		Responses:
//			200: registerResponse
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
		logic.ReturnErrorResponse(w, r, errorResponse)
		return
	}
	cert, ca, err := genCerts(&request.Key, &request.CommonName)
	if err != nil {
		logger.Log(0, "failed to generater certs ", err.Error())
		errorResponse := models.ErrorResponse{
			Code: http.StatusNotFound, Message: err.Error(),
		}
		logic.ReturnErrorResponse(w, r, errorResponse)
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
	logger.Log(2, r.Header.Get("user"),
		fmt.Sprintf("registered client [%+v] with server", request))
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// genCerts generates a client certificate and returns the certificate and root CA
func genCerts(clientKey *ed25519.PrivateKey, name *pkix.Name) (*x509.Certificate, *x509.Certificate, error) {
	ca, err := serverctl.ReadCertFromDB(tls.ROOT_PEM_NAME)
	if err != nil {
		logger.Log(2, "root ca not found ", err.Error())
		return nil, nil, fmt.Errorf("root ca not found %w", err)
	}
	key, err := serverctl.ReadKeyFromDB(tls.ROOT_KEY_NAME)
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
