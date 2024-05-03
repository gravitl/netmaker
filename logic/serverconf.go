package logic

import (
	"encoding/json"
	"time"

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
	// IngressesLimit - dummy var for community
	IngressesLimit = 1000000000
	// EgressesLimit - dummy var for community
	EgressesLimit = 1000000000
	// FreeTier - specifies if free tier
	FreeTier = false
	// DefaultTrialEndDate - is a placeholder date for not applicable trial end dates
	DefaultTrialEndDate, _ = time.Parse("2006-Jan-02", "2021-Apr-01")

	GetTrialEndDate = func() (time.Time, error) {
		return DefaultTrialEndDate, nil
	}
)

type serverData struct {
	PrivateKey string `json:"privatekey,omitempty" bson:"privatekey,omitempty"`
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
	NetworksLimit = servercfg.GetNetworkLimit()
	MachinesLimit = servercfg.GetMachinesLimit()
	IngressesLimit = servercfg.GetIngressLimit()
	EgressesLimit = servercfg.GetEgressLimit()
}
