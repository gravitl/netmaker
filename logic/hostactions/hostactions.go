package hostactions

import (
	"sync"

	"github.com/gravitl/netmaker/models"
)

// nodeActionHandler - handles the storage of host action updates
var nodeActionHandler sync.Map

// AddAction - adds a host action to a host's list to be retrieved from broker update
func AddAction(hu models.HostUpdate) {
	currentRecords, ok := nodeActionHandler.Load(hu.Host.ID.String())
	if !ok { // no list exists yet
		nodeActionHandler.Store(hu.Host.ID.String(), []models.HostUpdate{hu})
	} else { // list exists, append to it
		currentList := currentRecords.([]models.HostUpdate)
		currentList = append(currentList, hu)
		nodeActionHandler.Store(hu.Host.ID.String(), currentList)
	}
}

// GetAction - gets an action if exists
// TODO: may need to move to DB rather than sync map for HA
func GetAction(id string) *models.HostUpdate {
	currentRecords, ok := nodeActionHandler.Load(id)
	if !ok {
		return nil
	}
	currentList := currentRecords.([]models.HostUpdate)
	if len(currentList) > 0 {
		hu := currentList[0]
		nodeActionHandler.Store(hu.Host.ID.String(), currentList[1:])
		return &hu
	}
	return nil
}
