package logic

import (
	"encoding/json"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/servercfg"
)

var (
	// NetworksLimit - dummy var for community
	NetworksLimit = 1000000000
	// UsersLimit - dummy var for community
	UsersLimit = 1000000000
	// MachinesLimit - dummy var for community
	MachinesLimit = 1000000000
	// ClientsLimit - dummy var for community
	ClientsLimit = 1000000000
	// HostsLimit - dummy var for community
	HostsLimit = 1000000000
	// IngressesLimit - dummy var for community
	IngressesLimit = 1000000000
	// EgressesLimit - dummy var for community
	EgressesLimit = 1000000000
	// FreeTier - specifies if free tier
	FreeTier = false
)

type serverData struct {
	PrivateKey string `json:"privatekey,omitempty" bson:"privatekey,omitempty"`
}

// StorePrivKey - stores server client WireGuard privatekey if needed
func StorePrivKey(serverID string, privateKey string) error {
	var newData = serverData{}
	var err error
	var data []byte
	newData.PrivateKey = privateKey
	data, err = json.Marshal(&newData)
	if err != nil {
		return err
	}
	return database.Insert(serverID, string(data), database.SERVERCONF_TABLE_NAME)
}

// FetchPrivKey - fetches private key
func FetchPrivKey(serverID string) (string, error) {
	var dbData string
	var err error
	var fetchedData = serverData{}
	dbData, err = database.FetchRecord(database.SERVERCONF_TABLE_NAME, serverID)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal([]byte(dbData), &fetchedData)
	if err != nil {
		return "", err
	}
	return fetchedData.PrivateKey, nil
}

// RemovePrivKey - removes a private key
func RemovePrivKey(serverID string) error {
	return database.DeleteRecord(database.SERVERCONF_TABLE_NAME, serverID)
}

// FetchJWTSecret - fetches jwt secret from db
func FetchJWTSecret() (string, error) {
	var dbData string
	var err error
	var fetchedData = serverData{}
	dbData, err = database.FetchRecord(database.SERVERCONF_TABLE_NAME, "nm-jwt-secret")
	if err != nil {
		return "", err
	}
	err = json.Unmarshal([]byte(dbData), &fetchedData)
	if err != nil {
		return "", err
	}
	return fetchedData.PrivateKey, nil
}

// StoreJWTSecret - stores server jwt secret if needed
func StoreJWTSecret(privateKey string) error {
	var newData = serverData{}
	var err error
	var data []byte
	newData.PrivateKey = privateKey
	data, err = json.Marshal(&newData)
	if err != nil {
		return err
	}
	return database.Insert("nm-jwt-secret", string(data), database.SERVERCONF_TABLE_NAME)
}

// SetFreeTierLimits - sets limits for free tier
func SetFreeTierLimits() {
	FreeTier = true
	UsersLimit = servercfg.GetUserLimit()
	ClientsLimit = servercfg.GetClientLimit()
	NetworksLimit = servercfg.GetNetworkLimit()
	HostsLimit = servercfg.GetHostLimit()
	MachinesLimit = servercfg.GetMachinesLimit()
	IngressesLimit = servercfg.GetIngressLimit()
	EgressesLimit = servercfg.GetEgressLimit()
}
