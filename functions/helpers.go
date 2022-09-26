package functions

import (
	"encoding/json"
	"strings"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
)

// NameInDNSCharSet - name in dns char set
func NameInDNSCharSet(name string) bool {

	charset := "abcdefghijklmnopqrstuvwxyz1234567890-."

	for _, char := range name {
		if !strings.Contains(charset, strings.ToLower(string(char))) {
			return false
		}
	}
	return true
}

// NameInNodeCharSet - name in node char set
func NameInNodeCharSet(name string) bool {

	charset := "abcdefghijklmnopqrstuvwxyz1234567890-"

	for _, char := range name {
		if !strings.Contains(charset, strings.ToLower(string(char))) {
			return false
		}
	}
	return true
}

// RemoveDeletedNode - remove deleted node
func RemoveDeletedNode(nodeid string) bool {
	return database.DeleteRecord(database.DELETED_NODES_TABLE_NAME, nodeid) == nil
}

// GetAllExtClients - get all ext clients
func GetAllExtClients() ([]models.ExtClient, error) {
	var extclients []models.ExtClient
	collection, err := database.FetchRecords(database.EXT_CLIENT_TABLE_NAME)

	if err != nil {
		return extclients, err
	}

	for _, value := range collection {
		var extclient models.ExtClient
		err := json.Unmarshal([]byte(value), &extclient)
		if err != nil {
			return []models.ExtClient{}, err
		}
		// add node to our array
		extclients = append(extclients, extclient)
	}

	return extclients, nil
}
