package controller

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/kr/pretty"
	"golang.org/x/crypto/bcrypt"
)

// swagger:route PUT /api/nodes/{network}/{nodeid}/migrate nodes migrateNode
//
// Used to migrate a legacy node.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeJoinResponse
func migrate(w http.ResponseWriter, r *http.Request) {
	// we decode our body request params
	data := models.MigrationData{}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	params := mux.Vars(r)
	//check authorization
	record, err := database.FetchRecord(database.NODES_TABLE_NAME, data.LegacyNodeID)
	if err != nil {
		logger.Log(0, "no record for legacy node", data.LegacyNodeID, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	var legacyNode models.LegacyNode
	if err = json.Unmarshal([]byte(record), &legacyNode); err != nil {
		logger.Log(0, "error decoding legacy node", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(legacyNode.Password), []byte(data.Password)); err != nil {
		logger.Log(0, "error decoding legacy password", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "unauthorized"))
		return
	}
	network, err := logic.GetNetwork(params["network"])
	if err != nil {
		logger.Log(0, "error retrieving network:  ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	key, err := logic.CreateAccessKey(models.AccessKey{}, network)
	if err != nil {
		logger.Log(0, "error creating key:  ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	data.JoinData.Key = key.Value
	payload, err := json.Marshal(data.JoinData)
	if err != nil {
		logger.Log(0, "error encoding data:  ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	r.Body = io.NopCloser(strings.NewReader(string(payload)))
	r.ContentLength = int64(len(string(payload)))
	pretty.Println(data.JoinData)
	logger.Log(3, "deleteing legacy node", data.LegacyNodeID, legacyNode.ID, legacyNode.Name)
	if err := database.DeleteRecord(database.NODES_TABLE_NAME, data.LegacyNodeID); err != nil {
		logger.Log(0, "error deleting legacy node", legacyNode.Name, err.Error())
	}
	createNode(w, r)
	//newly created node has same node id as legacy node allowing using legacyNode.ID in gateway creation
	logger.Log(3, "re-creating legacy gateways")
	if legacyNode.IsIngressGateway == "yes" {
		if _, err := logic.CreateIngressGateway(legacyNode.Network, legacyNode.ID, false); err != nil {
			logger.Log(0, "error creating ingress gateway during migration", err.Error())
		}
	}
	if legacyNode.IsEgressGateway == "yes" {
		if _, err := logic.CreateEgressGateway(legacyNode.EgressGatewayRequest); err != nil {
			logger.Log(0, "error creating egress gateway during migration", err.Error())
		}
	}
	if legacyNode.IsRelay == "yes" {
		if _, _, err := logic.CreateRelay(models.RelayRequest{
			NodeID:     legacyNode.ID,
			NetID:      legacyNode.Network,
			RelayAddrs: legacyNode.RelayAddrs,
		}); err != nil {
			logger.Log(0, "error creating relay during migration", err.Error())
		}
	}
}
