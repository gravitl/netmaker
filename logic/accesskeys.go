package logic

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/rand"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

const (
	charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// CreateAccessKey - create access key
func CreateAccessKey(accesskey models.AccessKey, network models.Network) (models.AccessKey, error) {

	if accesskey.Name == "" {
		accesskey.Name = genKeyName()
	}

	if accesskey.Value == "" {
		accesskey.Value = genKey()
	}
	if accesskey.Uses == 0 {
		accesskey.Uses = 1
	}

	checkkeys, err := GetKeys(network.NetID)
	if err != nil {
		return models.AccessKey{}, errors.New("could not retrieve network keys")
	}

	for _, key := range checkkeys {
		if key.Name == accesskey.Name {
			return models.AccessKey{}, errors.New("duplicate AccessKey Name")
		}
	}
	privAddr := ""
	if network.IsLocal != "" {
		privAddr = network.LocalRange
	}

	netID := network.NetID

	var accessToken models.AccessToken
	s := servercfg.GetServerConfig()
	servervals := models.ServerConfig{
		CoreDNSAddr:     s.CoreDNSAddr,
		APIConnString:   s.APIConnString,
		APIHost:         s.APIHost,
		APIPort:         s.APIPort,
		GRPCConnString:  s.GRPCConnString,
		GRPCHost:        s.GRPCHost,
		GRPCPort:        s.GRPCPort,
		GRPCSSL:         s.GRPCSSL,
		CheckinInterval: s.CheckinInterval,
	}
	accessToken.ServerConfig = servervals
	accessToken.ClientConfig.Network = netID
	accessToken.ClientConfig.Key = accesskey.Value
	accessToken.ClientConfig.LocalRange = privAddr

	tokenjson, err := json.Marshal(accessToken)
	if err != nil {
		return accesskey, err
	}

	accesskey.AccessString = base64.StdEncoding.EncodeToString([]byte(tokenjson))

	//validate accesskey
	v := validator.New()
	err = v.Struct(accesskey)
	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			logger.Log(1, "validator", e.Error())
		}
		return models.AccessKey{}, err
	}

	network.AccessKeys = append(network.AccessKeys, accesskey)
	data, err := json.Marshal(&network)
	if err != nil {
		return models.AccessKey{}, err
	}
	if err = database.Insert(network.NetID, string(data), database.NETWORKS_TABLE_NAME); err != nil {
		return models.AccessKey{}, err
	}

	return accesskey, nil
}

// DeleteKey - deletes a key
func DeleteKey(keyname, netname string) error {
	network, err := GetParentNetwork(netname)
	if err != nil {
		return err
	}
	//basically, turn the list of access keys into the list of access keys before and after the item
	//have not done any error handling for if there's like...1 item. I think it works? need to test.
	found := false
	var updatedKeys []models.AccessKey
	for _, currentkey := range network.AccessKeys {
		if currentkey.Name == keyname {
			found = true
		} else {
			updatedKeys = append(updatedKeys, currentkey)
		}
	}
	if !found {
		return errors.New("key " + keyname + " does not exist")
	}
	network.AccessKeys = updatedKeys
	data, err := json.Marshal(&network)
	if err != nil {
		return err
	}
	if err := database.Insert(network.NetID, string(data), database.NETWORKS_TABLE_NAME); err != nil {
		return err
	}

	return nil
}

// GetKeys - fetches keys for network
func GetKeys(net string) ([]models.AccessKey, error) {

	record, err := database.FetchRecord(database.NETWORKS_TABLE_NAME, net)
	if err != nil {
		return []models.AccessKey{}, err
	}
	network, err := ParseNetwork(record)
	if err != nil {
		return []models.AccessKey{}, err
	}
	return network.AccessKeys, nil
}

// DecrimentKey - decriments key uses
func DecrimentKey(networkName string, keyvalue string) {

	var network models.Network

	network, err := GetParentNetwork(networkName)
	if err != nil {
		return
	}

	for i := len(network.AccessKeys) - 1; i >= 0; i-- {

		currentkey := network.AccessKeys[i]
		if currentkey.Value == keyvalue {
			network.AccessKeys[i].Uses--
			if network.AccessKeys[i].Uses < 1 {
				network.AccessKeys = append(network.AccessKeys[:i],
					network.AccessKeys[i+1:]...)
				break
			}
		}
	}

	if newNetworkData, err := json.Marshal(&network); err != nil {
		logger.Log(2, "failed to decrement key")
		return
	} else {
		database.Insert(network.NetID, string(newNetworkData), database.NETWORKS_TABLE_NAME)
	}
}

// IsKeyValid - check if key is valid
func IsKeyValid(networkname string, keyvalue string) bool {

	network, _ := GetParentNetwork(networkname)
	var key models.AccessKey
	foundkey := false
	isvalid := false

	for i := len(network.AccessKeys) - 1; i >= 0; i-- {
		currentkey := network.AccessKeys[i]
		if currentkey.Value == keyvalue {
			key = currentkey
			foundkey = true
		}
	}
	if foundkey {
		if key.Uses > 0 {
			isvalid = true
		}
	}
	return isvalid
}

// RemoveKeySensitiveInfo - remove sensitive key info
func RemoveKeySensitiveInfo(keys []models.AccessKey) []models.AccessKey {
	var returnKeys []models.AccessKey
	for _, key := range keys {
		key.Value = models.PLACEHOLDER_KEY_TEXT
		key.AccessString = models.PLACEHOLDER_TOKEN_TEXT
		returnKeys = append(returnKeys, key)
	}
	return returnKeys
}

// == private methods ==

func genKeyName() string {

	var seededRand *rand.Rand = rand.New(
		rand.NewSource(time.Now().UnixNano()))

	length := 5

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return "key" + string(b)
}

func genKey() string {

	var seededRand *rand.Rand = rand.New(
		rand.NewSource(time.Now().UnixNano()))

	length := 16

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}
