package controller

import (
	"encoding/json"
	"net/http"

	"github.com/gravitl/netmaker/auth"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/crypto/bcrypt"
)

// swagger:route PUT /api/v1/nodes/migrate nodes migrateNode
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
	data := models.MigrationData{}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	var networksToAdd = []string{}
	for i := range data.LegacyNodes {
		legacyNode := data.LegacyNodes[i]
		record, err := database.FetchRecord(database.NODES_TABLE_NAME, legacyNode.ID)
		if err != nil {
			logger.Log(0, "no record for legacy node", legacyNode.ID, err.Error())
			continue
		} else {
			var oldLegacyNode models.LegacyNode
			if err = json.Unmarshal([]byte(record), &oldLegacyNode); err != nil {
				logger.Log(0, "error decoding legacy node", err.Error())
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
				continue
			}
			if err := bcrypt.CompareHashAndPassword([]byte(oldLegacyNode.Password), []byte(legacyNode.Password)); err != nil {
				logger.Log(0, "error decoding legacy password", err.Error())
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, "unauthorized"))
				continue
			}
			networksToAdd = append(networksToAdd, oldLegacyNode.Network)
			_ = database.DeleteRecord(database.NODES_TABLE_NAME, oldLegacyNode.ID)
		}
	}
	if len(networksToAdd) == 0 {
		logger.Log(0, "no valid networks to migrate for host", data.NewHost.Name)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "unauthorized"))
		return
	}
	if !logic.HostExists(&data.NewHost) {
		logic.CheckHostPorts(&data.NewHost, true)
		if err = logic.CreateHost(&data.NewHost); err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
	}
	key, keyErr := logic.RetrievePublicTrafficKey()
	if keyErr != nil {
		logger.Log(0, "error retrieving key:", keyErr.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	server := servercfg.GetServerInfo()
	server.TrafficKey = key
	response := models.RegisterResponse{
		ServerConf:    server,
		RequestedHost: data.NewHost,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&response)
	logger.Log(0, "successfully migrated host", data.NewHost.Name, data.NewHost.ID.String())
	// notify host of changes, peer and node updates
	go auth.CheckNetRegAndHostUpdate(networksToAdd, &data.NewHost)
}
