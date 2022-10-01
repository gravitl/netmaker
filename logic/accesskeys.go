package logic

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"sync"

	validator "github.com/go-playground/validator/v10"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

// CreateAccessKey - create access key
func CreateAccessKey(accesskey models.AccessKey, network models.Network) (models.AccessKey, error) {

	if accesskey.Name == "" {
		accesskey.Name = genKeyName()
	}

	if accesskey.Value == "" {
		accesskey.Value = GenKey()
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

	accessToken.APIConnString = servercfg.GetAPIConnString()
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
func IsKeyValid(networkname string, keyvalue string) (string, bool) {

	network, err := GetParentNetwork(networkname)
	if err != nil {
		return "", false
	}
	accesskeys := network.AccessKeys

	var key models.AccessKey
	foundkey := false
	isvalid := false

	for i := len(accesskeys) - 1; i >= 0; i-- {
		currentkey := accesskeys[i]
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
	return key.Name, isvalid
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

const (
	maxr string = "ff578f57c15bb743beaa77d27637e02b598dffa9aebd15889187fe6eb3bdca516c3fa1a52eabef31f33b4b8c2e5b5524f1aa4f3329393912f40dbbe23d7f39723e0be05b6696b11f8eea0abe365a11d9f2735ac7e5b4e015ab19b35b84893685b37a9a0a62a566d6571d7e00d4241687f5c804f37cde9bf311c0781f51cc007c5a01a94f6cfcecea640b8e9ab7bd43e73e5df5d0e1eeb4d9b6cc44be67b7cad80808b17869561b579ffe0bbdeca5c83139e458000000000000000000000000000000000000000000000000000000000000000"
)

var (
	uno        sync.Once
	maxentropy *big.Int
)

func init() {
	uno.Do(func() {
		maxentropy, _ = new(big.Int).SetString(maxr, 16)
	})
}

// == private methods ==

func genKeyName() string {
	entropy, _ := rand.Int(rand.Reader, maxentropy)
	return strings.Join([]string{"key", entropy.Text(16)[:16]}, "-")
}

// GenKey - generates random key of length 16
func GenKey() string {
	entropy, _ := rand.Int(rand.Reader, maxentropy)
	return entropy.Text(16)[:16]
}

// GenPassWord - generates random password of length 64
func GenPassWord() string {
	entropy, _ := rand.Int(rand.Reader, maxentropy)
	return entropy.Text(62)[:64]
}
