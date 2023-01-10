package controller

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
)

// swagger:route POST /api/hosts/{hostid}/relay hosts createHostRelay
//
// Create a relay.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeResponse
func createHostRelay(w http.ResponseWriter, r *http.Request) {
	var relay models.HostRelayRequest
	var params = mux.Vars(r)
	w.Header().Set("Content-Type", "application/json")
	err := json.NewDecoder(r.Body).Decode(&relay)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	relay.HostID = params["hostid"]
	relayHost, _, err := logic.CreateHostRelay(relay)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to create relay on host [%s]: %v", relay.HostID, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	logger.Log(1, r.Header.Get("user"), "created relay on host", relay.HostID)
	go func(relayHostID string) {
		relatedhosts := logic.GetRelatedHosts(relayHostID)
		for _, relatedHost := range relatedhosts {
			relatedHost.ProxyEnabled = true
			logic.UpsertHost(&relatedHost)
		}
		if err := mq.PublishPeerUpdate(); err != nil {
			logger.Log(0, "fail to publish peer update: ", err.Error())
		}

	}(relay.HostID)

	apiHostData := relayHost.ConvertNMHostToAPI()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiHostData)
}

// swagger:route DELETE /api/hosts/{hostid}/relay hosts deleteHostRelay
//
// Remove a relay.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeResponse
func deleteHostRelay(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	hostid := params["hostid"]
	relayHost, _, err := logic.DeleteHostRelay(hostid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "deleted relay host", hostid)
	go func() {
		if err := mq.PublishPeerUpdate(); err != nil {
			logger.Log(0, "fail to publish peer update: ", err.Error())
		}
	}()
	apiHostData := relayHost.ConvertNMHostToAPI()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiHostData)
}
