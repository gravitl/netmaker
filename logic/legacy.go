package logic

import (
	"encoding/json"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
)

// IsLegacyNode - checks if a node is legacy or not
func IsLegacyNode(nodeID string) bool {
	record, err := database.FetchRecord(database.NODES_TABLE_NAME, nodeID)
	if err != nil {
		return false
	}
	var currentNode models.Node
	var legacyNode models.LegacyNode
	currentNodeErr := json.Unmarshal([]byte(record), &currentNode)
	legacyNodeErr := json.Unmarshal([]byte(record), &legacyNode)
	return currentNodeErr != nil && legacyNodeErr == nil
}

// CheckAndRemoveLegacyNode - checks for legacy node and removes
func CheckAndRemoveLegacyNode(nodeID string) bool {
	if IsLegacyNode(nodeID) {
		if err := database.DeleteRecord(database.NODES_TABLE_NAME, nodeID); err == nil {
			return true
		}
	}
	return false
}

// RemoveAllLegacyNodes - fetches all legacy nodes from DB and removes
func RemoveAllLegacyNodes() error {
	records, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil {
		return err
	}
	for k := range records {
		if CheckAndRemoveLegacyNode(k) {
			logger.Log(0, "removed legacy node", k)
		}
	}
	return nil
}
