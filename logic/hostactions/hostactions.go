package hostactions

import (
	"encoding/json"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
)

// AddAction - adds a host action to a host's list to be retrieved from broker update
func AddAction(hu models.HostUpdate) {
	hostID := hu.Host.ID.String()
	currentRecords, err := database.FetchRecord(database.HOST_ACTIONS_TABLE_NAME, hostID)
	if err != nil {
		if database.IsEmptyRecord(err) { // no list exists yet
			newEntry, err := json.Marshal([]models.HostUpdate{hu})
			if err != nil {
				return
			}
			_ = database.Insert(hostID, string(newEntry), database.HOST_ACTIONS_TABLE_NAME)
		}
		return
	}
	var currentList []models.HostUpdate
	if err := json.Unmarshal([]byte(currentRecords), &currentList); err != nil {
		return
	}
	currentList = append(currentList, hu)
	newData, err := json.Marshal(currentList)
	if err != nil {
		return
	}
	_ = database.Insert(hostID, string(newData), database.HOST_ACTIONS_TABLE_NAME)
}

// GetAction - gets an action if exists
func GetAction(id string) *models.HostUpdate {
	currentRecords, err := database.FetchRecord(database.HOST_ACTIONS_TABLE_NAME, id)
	if err != nil {
		return nil
	}
	var currentList []models.HostUpdate
	if err = json.Unmarshal([]byte(currentRecords), &currentList); err != nil {
		return nil
	}
	if len(currentList) > 0 {
		hu := currentList[0]
		newData, err := json.Marshal(currentList[1:])
		if err != nil {
			newData, _ = json.Marshal([]models.HostUpdate{})
		}
		_ = database.Insert(id, string(newData), database.HOST_ACTIONS_TABLE_NAME)
		return &hu
	}
	return nil
}
