package controller

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/servercfg"
)

func enrollmentKeyHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/enrollment-keys", logic.SecurityCheck(true, http.HandlerFunc(getEnrollmentKeys))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/enrollment-keys/{keyID}", logic.SecurityCheck(true, http.HandlerFunc(deleteEnrollmentKey))).Methods(http.MethodDelete)
	r.HandleFunc("/api/v1/host/register", logic.SecurityCheck(true, http.HandlerFunc(handleHostRegister))).Methods(http.MethodPost)
}

// swagger:route GET /api/v1/enrollment-keys enrollmentKeys getEnrollmentKeys
//
// Lists all hosts.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: getEnrollmentKeysSlice
func getEnrollmentKeys(w http.ResponseWriter, r *http.Request) {
	currentKeys, err := logic.GetAllEnrollmentKeys()
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to fetch enrollment keys: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	for i := range currentKeys {
		if err = logic.Tokenize(currentKeys[i], servercfg.GetServer()); err != nil {
			logger.Log(0, r.Header.Get("user"), "failed to get token values for keys:", err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
	}
	// return JSON/API formatted hosts
	logger.Log(2, r.Header.Get("user"), "fetched enrollment keys")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(currentKeys)
}

// swagger:route DELETE /api/v1/enrollment-keys/{keyID} enrollmentKeys deleteEnrollmentKey
//
// Deletes a Netclient host from Netmaker server.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: deleteEnrollmentKeyResponse
func deleteEnrollmentKey(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	keyID := params["keyID"]
	err := logic.DeleteEnrollmentKey(keyID)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to remove enrollment key: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(2, r.Header.Get("user"), "deleted enrollment key", keyID)
	w.WriteHeader(http.StatusOK)
}

// swagger:route DELETE /api/v1/enrollment-keys/{keyID} enrollmentKeys deleteEnrollmentKey
//
// Deletes a Netclient host from Netmaker server.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: hostRegisterResponse
func handleHostRegister(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	keyID := params["keyID"]
	err := logic.DeleteEnrollmentKey(keyID)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to remove enrollment key: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(2, r.Header.Get("user"), "deleted enrollment key", keyID)
	w.WriteHeader(http.StatusOK)
}
