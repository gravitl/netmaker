package logic

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
	"golang.org/x/exp/slog"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var (
	extClientCacheMutex = &sync.RWMutex{}
	extClientCacheMap   = make(map[string]models.ExtClient)
)

func getAllExtClientsFromCache() (extClients []models.ExtClient) {
	extClientCacheMutex.RLock()
	for _, extclient := range extClientCacheMap {
		extClients = append(extClients, extclient)
	}
	extClientCacheMutex.RUnlock()
	return
}

func deleteExtClientFromCache(key string) {
	extClientCacheMutex.Lock()
	delete(extClientCacheMap, key)
	extClientCacheMutex.Unlock()
}

func getExtClientFromCache(key string) (extclient models.ExtClient, ok bool) {
	extClientCacheMutex.RLock()
	extclient, ok = extClientCacheMap[key]
	extClientCacheMutex.RUnlock()
	return
}

func storeExtClientInCache(key string, extclient models.ExtClient) {
	extClientCacheMutex.Lock()
	extClientCacheMap[key] = extclient
	extClientCacheMutex.Unlock()
}

// ExtClient.GetEgressRangesOnNetwork - returns the egress ranges on network of ext client
func GetEgressRangesOnNetwork(client *models.ExtClient) ([]string, error) {

	var result []string
	networkNodes, err := GetNetworkNodes(client.Network)
	if err != nil {
		return []string{}, err
	}
	for _, currentNode := range networkNodes {
		if currentNode.Network != client.Network {
			continue
		}
		if currentNode.IsEgressGateway { // add the egress gateway range(s) to the result
			if len(currentNode.EgressGatewayRanges) > 0 {
				result = append(result, currentNode.EgressGatewayRanges...)
			}
		}
	}

	return result, nil
}

// DeleteExtClient - deletes an existing ext client
func DeleteExtClient(network string, clientid string) error {
	key, err := GetRecordKey(clientid, network)
	if err != nil {
		return err
	}
	err = database.DeleteRecord(database.EXT_CLIENT_TABLE_NAME, key)
	if err != nil {
		if database.IsEmptyRecord(err) {
			deleteExtClientFromCache(key)
		}
		return err
	}
	deleteExtClientFromCache(key)
	return nil
}

// GetNetworkExtClients - gets the ext clients of given network
func GetNetworkExtClients(network string) ([]models.ExtClient, error) {
	var extclients []models.ExtClient
	allextclients := getAllExtClientsFromCache()
	if len(allextclients) != 0 {
		for _, extclient := range allextclients {
			if extclient.Network == network {
				extclients = append(extclients, extclient)
			}
		}
		return extclients, nil
	}
	records, err := database.FetchRecords(database.EXT_CLIENT_TABLE_NAME)
	if err != nil {
		if database.IsEmptyRecord(err) {
			return extclients, nil
		}
		return extclients, err
	}
	for _, value := range records {
		var extclient models.ExtClient
		err = json.Unmarshal([]byte(value), &extclient)
		if err != nil {
			continue
		}
		key, err := GetRecordKey(extclient.ClientID, network)
		if err == nil {
			storeExtClientInCache(key, extclient)
		}
		if extclient.Network == network {
			extclients = append(extclients, extclient)
		}
	}
	return extclients, err
}

// GetExtClient - gets a single ext client on a network
func GetExtClient(clientid string, network string) (models.ExtClient, error) {
	var extclient models.ExtClient
	key, err := GetRecordKey(clientid, network)
	if err != nil {
		return extclient, err
	}
	if extclient, ok := getExtClientFromCache(key); ok {
		return extclient, nil
	}
	data, err := database.FetchRecord(database.EXT_CLIENT_TABLE_NAME, key)
	if err != nil {
		return extclient, err
	}
	err = json.Unmarshal([]byte(data), &extclient)
	storeExtClientInCache(key, extclient)
	return extclient, err
}

// GetExtClient - gets a single ext client on a network
func GetExtClientByPubKey(publicKey string, network string) (*models.ExtClient, error) {
	netClients, err := GetNetworkExtClients(network)
	if err != nil {
		return nil, err
	}
	for i := range netClients {
		ec := netClients[i]
		if ec.PublicKey == publicKey {
			return &ec, nil
		}
	}

	return nil, fmt.Errorf("no client found")
}

// CreateExtClient - creates and saves an extclient
func CreateExtClient(extclient *models.ExtClient) error {
	// lock because we may need unique IPs and having it concurrent makes parallel calls result in same "unique" IPs
	addressLock.Lock()
	defer addressLock.Unlock()

	if len(extclient.PublicKey) == 0 {
		privateKey, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return err
		}
		extclient.PrivateKey = privateKey.String()
		extclient.PublicKey = privateKey.PublicKey().String()
	} else if len(extclient.PrivateKey) == 0 && len(extclient.PublicKey) > 0 {
		extclient.PrivateKey = "[ENTER PRIVATE KEY]"
	}

	parentNetwork, err := GetNetwork(extclient.Network)
	if err != nil {
		return err
	}
	if extclient.Address == "" {
		if parentNetwork.IsIPv4 == "yes" {
			newAddress, err := UniqueAddress(extclient.Network, true)
			if err != nil {
				return err
			}
			extclient.Address = newAddress.String()
		}
	}

	if extclient.Address6 == "" {
		if parentNetwork.IsIPv6 == "yes" {
			addr6, err := UniqueAddress6(extclient.Network, true)
			if err != nil {
				return err
			}
			extclient.Address6 = addr6.String()
		}
	}

	if extclient.ClientID == "" {
		extclient.ClientID = models.GenerateNodeName()
	}

	extclient.LastModified = time.Now().Unix()
	return SaveExtClient(extclient)
}

// SaveExtClient - saves an ext client to database
func SaveExtClient(extclient *models.ExtClient) error {
	key, err := GetRecordKey(extclient.ClientID, extclient.Network)
	if err != nil {
		return err
	}
	data, err := json.Marshal(&extclient)
	if err != nil {
		return err
	}
	if err = database.Insert(key, string(data), database.EXT_CLIENT_TABLE_NAME); err != nil {
		return err
	}
	storeExtClientInCache(key, *extclient)
	return SetNetworkNodesLastModified(extclient.Network)
}

// UpdateExtClient - updates an ext client with new values
func UpdateExtClient(old *models.ExtClient, update *models.CustomExtClient) models.ExtClient {
	new := *old
	new.ClientID = update.ClientID
	if update.PublicKey != "" && old.PublicKey != update.PublicKey {
		new.PublicKey = update.PublicKey
	}
	if update.DNS != old.DNS {
		new.DNS = update.DNS
	}
	if update.Enabled != old.Enabled {
		new.Enabled = update.Enabled
	}
	if update.ExtraAllowedIPs != nil && StringDifference(old.ExtraAllowedIPs, update.ExtraAllowedIPs) != nil {
		new.ExtraAllowedIPs = update.ExtraAllowedIPs
	}
	if update.DeniedACLs != nil && !reflect.DeepEqual(old.DeniedACLs, update.DeniedACLs) {
		new.DeniedACLs = update.DeniedACLs
	}
	return new
}

// GetExtClientsByID - gets the clients of attached gateway
func GetExtClientsByID(nodeid, network string) ([]models.ExtClient, error) {
	var result []models.ExtClient
	currentClients, err := GetNetworkExtClients(network)
	if err != nil {
		return result, err
	}
	for i := range currentClients {
		if currentClients[i].IngressGatewayID == nodeid {
			result = append(result, currentClients[i])
		}
	}
	return result, nil
}

// GetAllExtClients - gets all ext clients from DB
func GetAllExtClients() ([]models.ExtClient, error) {
	var clients = []models.ExtClient{}
	currentNetworks, err := GetNetworks()
	if err != nil && database.IsEmptyRecord(err) {
		return clients, nil
	} else if err != nil {
		return clients, err
	}

	for i := range currentNetworks {
		netName := currentNetworks[i].NetID
		netClients, err := GetNetworkExtClients(netName)
		if err != nil {
			continue
		}
		clients = append(clients, netClients...)
	}

	return clients, nil
}

// ToggleExtClientConnectivity - enables or disables an ext client
func ToggleExtClientConnectivity(client *models.ExtClient, enable bool) (models.ExtClient, error) {
	update := models.CustomExtClient{
		Enabled:              enable,
		ClientID:             client.ClientID,
		PublicKey:            client.PublicKey,
		DNS:                  client.DNS,
		ExtraAllowedIPs:      client.ExtraAllowedIPs,
		DeniedACLs:           client.DeniedACLs,
		RemoteAccessClientID: client.RemoteAccessClientID,
	}

	// update in DB
	newClient := UpdateExtClient(client, &update)
	if err := DeleteExtClient(client.Network, client.ClientID); err != nil {
		slog.Error("failed to delete ext client during update", "id", client.ClientID, "network", client.Network, "error", err)
		return newClient, err
	}
	if err := SaveExtClient(&newClient); err != nil {
		slog.Error("failed to save updated ext client during update", "id", newClient.ClientID, "network", newClient.Network, "error", err)
		return newClient, err
	}

	return newClient, nil
}
