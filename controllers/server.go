package controller

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/tls"
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
	logger.Log(2, "processing registration request")
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
		logger.Log(0, "error decoding request", err.Error())
		errorResponse := models.ErrorResponse{
			Code: http.StatusBadRequest, Message: err.Error(),
		}
		returnErrorResponse(w, r, errorResponse)
		return
	}
	request.CSR.PublicKey = request.Key
	found := false
	networks, err := logic.GetNetworks()
	if err != nil {
		logger.Log(0, "no networks", err.Error())
		errorResponse := models.ErrorResponse{
			Code: http.StatusNotFound, Message: "no networks",
		}
		returnErrorResponse(w, r, errorResponse)
		return
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
		logger.Log(0, "valid access key not found")
		errorResponse := models.ErrorResponse{
			Code: http.StatusUnauthorized, Message: "You are unauthorized to access this endpoint.",
		}
		returnErrorResponse(w, r, errorResponse)
		return
	}
	// not working --- use openssl instead
	//	cert, ca, err := genCerts(&request.CSR, request.Key)
	key, cert, ca, err := genOpenSSLCerts()
	if err != nil {
		logger.Log(0, "failed to generater certs ", err.Error())
		errorResponse := models.ErrorResponse{
			Code: http.StatusNotFound, Message: err.Error(),
		}
		returnErrorResponse(w, r, errorResponse)
		return
	}

	// caBytes, err := config.ConvertCertToBytes(*ca)
	// if err != nil {
	// 	logger.Log(0, "failed to encode CA cert ", err.Error())
	// 	errorResponse := models.ErrorResponse{
	// 		Code: http.StatusInternalServerError, Message: err.Error(),
	// 	}
	// 	returnErrorResponse(w, r, errorResponse)
	// 	return
	// }

	// certBytes, err := config.ConvertCertToBytes(*cert)
	// if err != nil {
	// 	logger.Log(0, "failed to encode CA cert ", err.Error())
	// 	errorResponse := models.ErrorResponse{
	// 		Code: http.StatusInternalServerError, Message: err.Error(),
	// 	}
	// 	returnErrorResponse(w, r, errorResponse)
	// 	return
	// }

	tls.SaveCert("/tmp/sent/", "root.pem", ca)
	tls.SaveCert("/tmp/sent/", "client.pem", cert)
	tls.SaveKey("/tmp/sent/", "client.key", *key)
	response := config.RegisterResponse{
		Key:        *key,
		CA:         *ca,
		CAPubKey:   (ca.PublicKey).(ed25519.PublicKey),
		Cert:       *cert,
		CertPubKey: (cert.PublicKey).(ed25519.PublicKey),
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func genCerts(csr *x509.CertificateRequest, publickey ed25519.PublicKey) (*x509.Certificate, *x509.Certificate, error) {
	ca, err := tls.ReadCert("/etc/netmaker/server.pem")
	if err != nil {
		logger.Log(2, "root ca not found ", err.Error())
		return nil, nil, fmt.Errorf("root ca not found %w", err)
	}
	key, err := tls.ReadKey("/etc/netmaker/server.key")
	if err != nil {
		logger.Log(2, "root key not found ", err.Error())
		return nil, nil, fmt.Errorf("root key not found %w", err)
	}
	//_, privKey, err := ed25519.GenerateKey(rand.Reader)
	//if err != nil {
	//	logger.Log(2, "failed to generate client key", err.Error())
	//	return nil, nil, nil, fmt.Errorf("client key generation failed %w", err)
	//}
	//csr, err := tls.NewCSR(privKey, name)
	//if err != nil {
	//	logger.Log(2, "failed to generate client certificate requests", err.Error())
	//	return nil, nil, nil, fmt.Errorf("client certification request generation failed %w", err)
	//}

	csr.PublicKey = publickey
	cert, err := tls.NewEndEntityCert(*key, csr, ca, tls.CERTIFICATE_VALIDITY)
	if err != nil {
		logger.Log(2, "unable to generate client certificate", err.Error())
		return nil, nil, fmt.Errorf("client certification generation failed %w", err)
	}
	return ca, cert, nil
}

func genOpenSSLCerts() (*ed25519.PrivateKey, *x509.Certificate, *x509.Certificate, error) {
	cmd1 := "openssl genpkey -algorithm Ed25519 -out /tmp/client.key"
	cmd2 := "openssl req -new -out /tmp/client.csr -key /tmp/client.key -subj /CN=client"
	cmd3 := "openssl x509 -req -in /tmp/client.csr -days 365 -CA /etc/netmaker/root.pem -CAkey /etc/netmaker/root.key -CAcreateserial -out /tmp/client.pem"

	if _, err := ncutils.RunCmd(cmd1, true); err != nil {
		return nil, nil, nil, fmt.Errorf("client key error %w", err)
	}
	if _, err := ncutils.RunCmd(cmd2, true); err != nil {
		return nil, nil, nil, fmt.Errorf("client csr error %w", err)
	}
	if _, err := ncutils.RunCmd(cmd3, true); err != nil {
		return nil, nil, nil, fmt.Errorf("client cert error %w", err)
	}
	key, err := tls.ReadKey("/tmp/client.key")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read client key error %w", err)
	}
	cert, err := tls.ReadCert("/tmp/client.pem")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read client cert error %w", err)
	}
	ca, err := tls.ReadCert("/etc/netmaker/root.pem")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read ca cert error %w", err)
	}
	return key, cert, ca, nil
}
