package logic

import "github.com/gravitl/netmaker/database"

// StorePrivKey - stores server client WireGuard privatekey if needed
func StorePrivKey(serverID string, privateKey string) error {
	return database.Insert(serverID, privateKey, database.SERVERCONF_TABLE_NAME)
}

func FetchPrivKey(serverID string) (string, error) {
	return database.FetchRecord(database.SERVERCONF_TABLE_NAME, serverID)
}

func RemovePrivKey(serverID string) error {
	return database.DeleteRecord(database.SERVERCONF_TABLE_NAME, serverID)
}
