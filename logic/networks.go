package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/c-robinson/iplib"
	validator "github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic/acls/nodeacls"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/validation"
	"golang.org/x/exp/slog"
)

var (
	networkCacheMutex = &sync.RWMutex{}
	networkCacheMap   = make(map[string]models.Network)
	allocatedIpMap    = make(map[string]map[string]net.IP)
)

// SetAllocatedIpMap - set allocated ip map for networks
func SetAllocatedIpMap() error {
	logger.Log(0, "start setting up allocated ip map")
	if allocatedIpMap == nil {
		allocatedIpMap = map[string]map[string]net.IP{}
	}

	currentNetworks, err := GetNetworks()
	if err != nil {
		return err
	}

	for _, v := range currentNetworks {
		pMap := map[string]net.IP{}
		netName := v.NetID

		//nodes
		nodes, err := GetNetworkNodes(netName)
		if err != nil {
			slog.Error("could not load node for network", netName, "error", err.Error())
		} else {
			for _, n := range nodes {

				if n.Address.IP != nil {
					pMap[n.Address.IP.String()] = n.Address.IP
				}
				if n.Address6.IP != nil {
					pMap[n.Address6.IP.String()] = n.Address6.IP
				}
			}

		}

		//extClients
		extClients, err := GetNetworkExtClients(netName)
		if err != nil {
			slog.Error("could not load extClient for network", netName, "error", err.Error())
		} else {
			for _, extClient := range extClients {
				if extClient.Address != "" {
					pMap[extClient.Address] = net.ParseIP(extClient.Address)
				}
				if extClient.Address6 != "" {
					pMap[extClient.Address6] = net.ParseIP(extClient.Address6)
				}
			}
		}

		allocatedIpMap[netName] = pMap
	}
	logger.Log(0, "setting up allocated ip map done")
	return nil
}

// ClearAllocatedIpMap - set allocatedIpMap to nil
func ClearAllocatedIpMap() {
	allocatedIpMap = nil
}

func AddIpToAllocatedIpMap(networkName string, ip net.IP) {
	networkCacheMutex.Lock()
	allocatedIpMap[networkName][ip.String()] = ip
	networkCacheMutex.Unlock()
}

func RemoveIpFromAllocatedIpMap(networkName string, ip string) {
	networkCacheMutex.Lock()
	delete(allocatedIpMap[networkName], ip)
	networkCacheMutex.Unlock()
}

// AddNetworkToAllocatedIpMap - add network to allocated ip map when network is added
func AddNetworkToAllocatedIpMap(networkName string) {
	networkCacheMutex.Lock()
	allocatedIpMap[networkName] = map[string]net.IP{}
	networkCacheMutex.Unlock()
}

// RemoveNetworkFromAllocatedIpMap - remove network from allocated ip map when network is deleted
func RemoveNetworkFromAllocatedIpMap(networkName string) {
	networkCacheMutex.Lock()
	delete(allocatedIpMap, networkName)
	networkCacheMutex.Unlock()
}

func getNetworksFromCache() (networks []models.Network) {
	networkCacheMutex.RLock()
	for _, network := range networkCacheMap {
		networks = append(networks, network)
	}
	networkCacheMutex.RUnlock()
	return
}

func deleteNetworkFromCache(key string) {
	networkCacheMutex.Lock()
	delete(networkCacheMap, key)
	networkCacheMutex.Unlock()
}

func getNetworkFromCache(key string) (network models.Network, ok bool) {
	networkCacheMutex.RLock()
	network, ok = networkCacheMap[key]
	networkCacheMutex.RUnlock()
	return
}

func storeNetworkInCache(key string, network models.Network) {
	networkCacheMutex.Lock()
	networkCacheMap[key] = network
	networkCacheMutex.Unlock()
}

// GetNetworks - returns all networks from database
func GetNetworks() ([]models.Network, error) {
	var networks []models.Network
	if servercfg.CacheEnabled() {
		networks := getNetworksFromCache()
		if len(networks) != 0 {
			return networks, nil
		}
	}
	collection, err := database.FetchRecords(database.NETWORKS_TABLE_NAME)
	if err != nil {
		return networks, err
	}

	for _, value := range collection {
		var network models.Network
		if err := json.Unmarshal([]byte(value), &network); err != nil {
			return networks, err
		}
		// add network our array
		networks = append(networks, network)
		if servercfg.CacheEnabled() {
			storeNetworkInCache(network.NetID, network)
		}
	}

	return networks, err
}

// DeleteNetwork - deletes a network
func DeleteNetwork(network string) error {
	// remove ACL for network
	err := nodeacls.DeleteACLContainer(nodeacls.NetworkID(network))
	if err != nil {
		logger.Log(1, "failed to remove the node acls during network delete for network,", network)
	}
	// Delete default network enrollment key
	keys, _ := GetAllEnrollmentKeys()
	for _, key := range keys {
		if key.Tags[0] == network {
			if key.Default {
				DeleteEnrollmentKey(key.Value, true)
				break
			}

		}
	}
	nodeCount, err := GetNetworkNonServerNodeCount(network)
	if nodeCount == 0 || database.IsEmptyRecord(err) {
		// delete server nodes first then db records
		err = database.DeleteRecord(database.NETWORKS_TABLE_NAME, network)
		if err != nil {
			return err
		}
		if servercfg.CacheEnabled() {
			deleteNetworkFromCache(network)
		}
		return nil
	}
	return errors.New("node check failed. All nodes must be deleted before deleting network")
}

// CreateNetwork - creates a network in database
func CreateNetwork(network models.Network) (models.Network, error) {
	network.NetID = fmt.Sprintf("%d", time.Now().Unix())
	if network.AddressRange != "" {
		normalizedRange, err := NormalizeCIDR(network.AddressRange)
		if err != nil {
			return models.Network{}, err
		}
		network.AddressRange = normalizedRange
	}
	if network.AddressRange6 != "" {
		normalizedRange, err := NormalizeCIDR(network.AddressRange6)
		if err != nil {
			return models.Network{}, err
		}
		network.AddressRange6 = normalizedRange
	}
	if !IsNetworkCIDRUnique(network.GetNetworkNetworkCIDR4(), network.GetNetworkNetworkCIDR6()) {
		return models.Network{}, errors.New("network cidr already in use")
	}

	network.SetDefaults()
	network.SetNodesLastModified()
	network.SetNetworkLastModified()

	err := ValidateNetwork(&network, false)
	if err != nil {
		//logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return models.Network{}, err
	}

	data, err := json.Marshal(&network)
	if err != nil {
		return models.Network{}, err
	}

	if err = database.Insert(network.NetID, string(data), database.NETWORKS_TABLE_NAME); err != nil {
		return models.Network{}, err
	}
	if servercfg.CacheEnabled() {
		storeNetworkInCache(network.NetID, network)
	}

	_, _ = CreateEnrollmentKey(
		0,
		time.Time{},
		[]string{network.NetID},
		[]string{network.NetID},
		[]models.TagID{},
		true,
		uuid.Nil,
		true,
	)

	return network, nil
}

// GetNetworkNonServerNodeCount - get number of network non server nodes
func GetNetworkNonServerNodeCount(networkName string) (int, error) {
	nodes, err := GetNetworkNodes(networkName)
	return len(nodes), err
}

func IsNetworkCIDRUnique(cidr4 *net.IPNet, cidr6 *net.IPNet) bool {
	networks, err := GetNetworks()
	if err != nil {
		return database.IsEmptyRecord(err)
	}
	for _, network := range networks {
		if intersect(network.GetNetworkNetworkCIDR4(), cidr4) ||
			intersect(network.GetNetworkNetworkCIDR6(), cidr6) {
			return false
		}
	}
	return true
}

func intersect(n1, n2 *net.IPNet) bool {
	if n1 == nil || n2 == nil {
		return false
	}
	return n2.Contains(n1.IP) || n1.Contains(n2.IP)
}

// GetParentNetwork - get parent network
func GetParentNetwork(networkname string) (models.Network, error) {

	var network models.Network
	if servercfg.CacheEnabled() {
		if network, ok := getNetworkFromCache(networkname); ok {
			return network, nil
		}
	}
	networkData, err := database.FetchRecord(database.NETWORKS_TABLE_NAME, networkname)
	if err != nil {
		return network, err
	}
	if err = json.Unmarshal([]byte(networkData), &network); err != nil {
		return models.Network{}, err
	}
	return network, nil
}

// GetNetworkSettings - get parent network
func GetNetworkSettings(networkname string) (models.Network, error) {

	var network models.Network
	if servercfg.CacheEnabled() {
		if network, ok := getNetworkFromCache(networkname); ok {
			return network, nil
		}
	}
	networkData, err := database.FetchRecord(database.NETWORKS_TABLE_NAME, networkname)
	if err != nil {
		return network, err
	}
	if err = json.Unmarshal([]byte(networkData), &network); err != nil {
		return models.Network{}, err
	}
	return network, nil
}

// UniqueAddress - get a unique ipv4 address
func UniqueAddress(networkName string, reverse bool) (net.IP, error) {
	add := net.IP{}
	var network models.Network
	network, err := GetParentNetwork(networkName)
	if err != nil {
		logger.Log(0, "UniqueAddressServer encountered  an error")
		return add, err
	}

	if network.IsIPv4 == "no" {
		return add, fmt.Errorf("IPv4 not active on network " + networkName)
	}
	//ensure AddressRange is valid
	if _, _, err := net.ParseCIDR(network.AddressRange); err != nil {
		logger.Log(0, "UniqueAddress encountered  an error")
		return add, err
	}
	net4 := iplib.Net4FromStr(network.AddressRange)
	newAddrs := net4.FirstAddress()

	if reverse {
		newAddrs = net4.LastAddress()
	}

	ipAllocated := allocatedIpMap[networkName]
	for {
		if _, ok := ipAllocated[newAddrs.String()]; !ok {
			return newAddrs, nil
		}
		if reverse {
			newAddrs, err = net4.PreviousIP(newAddrs)
		} else {
			newAddrs, err = net4.NextIP(newAddrs)
		}
		if err != nil {
			break
		}
	}

	return add, errors.New("ERROR: No unique addresses available. Check network subnet")
}

// IsIPUnique - checks if an IP is unique
func IsIPUnique(network string, ip string, tableName string, isIpv6 bool) bool {

	isunique := true
	if tableName == database.NODES_TABLE_NAME {
		nodes, err := GetNetworkNodes(network)
		if err != nil {
			return isunique
		}
		for _, node := range nodes {
			if isIpv6 {
				if node.Address6.IP.String() == ip && node.Network == network {
					return false
				}
			} else {
				if node.Address.IP.String() == ip && node.Network == network {
					return false
				}
			}
		}

	} else if tableName == database.EXT_CLIENT_TABLE_NAME {

		extClients, err := GetNetworkExtClients(network)
		if err != nil {
			return isunique
		}
		for _, extClient := range extClients { // filter
			if isIpv6 {
				if (extClient.Address6 == ip) && extClient.Network == network {
					return false
				}

			} else {
				if (extClient.Address == ip) && extClient.Network == network {
					return false
				}
			}
		}
	}

	return isunique
}

// UniqueAddress6 - see if ipv6 address is unique
func UniqueAddress6(networkName string, reverse bool) (net.IP, error) {
	add := net.IP{}
	var network models.Network
	network, err := GetParentNetwork(networkName)
	if err != nil {
		fmt.Println("Network Not Found")
		return add, err
	}
	if network.IsIPv6 == "no" {
		return add, fmt.Errorf("IPv6 not active on network " + networkName)
	}

	//ensure AddressRange is valid
	if _, _, err := net.ParseCIDR(network.AddressRange6); err != nil {
		return add, err
	}
	net6 := iplib.Net6FromStr(network.AddressRange6)

	newAddrs, err := net6.NextIP(net6.FirstAddress())
	if reverse {
		newAddrs, err = net6.PreviousIP(net6.LastAddress())
	}
	if err != nil {
		return add, err
	}

	ipAllocated := allocatedIpMap[networkName]
	for {
		if _, ok := ipAllocated[newAddrs.String()]; !ok {
			return newAddrs, nil
		}
		if reverse {
			newAddrs, err = net6.PreviousIP(newAddrs)
		} else {
			newAddrs, err = net6.NextIP(newAddrs)
		}
		if err != nil {
			break
		}
	}

	return add, errors.New("ERROR: No unique IPv6 addresses available. Check network subnet")
}

// IsNetworkNameUnique - checks to see if any other networks have the same name (id)
func IsNetworkNameUnique(network *models.Network) (bool, error) {

	isunique := true

	dbs, err := GetNetworks()

	if err != nil && !database.IsEmptyRecord(err) {
		return false, err
	}

	for i := 0; i < len(dbs); i++ {

		if network.NetID == dbs[i].NetID {
			isunique = false
		}
	}

	return isunique, nil
}

// UpdateNetwork - updates a network with another network's fields
func UpdateNetwork(currentNetwork *models.Network, newNetwork *models.Network) (bool, bool, bool, error) {
	if err := ValidateNetwork(newNetwork, true); err != nil {
		return false, false, false, err
	}
	if newNetwork.NetID == currentNetwork.NetID {
		hasrangeupdate4 := newNetwork.AddressRange != currentNetwork.AddressRange
		hasrangeupdate6 := newNetwork.AddressRange6 != currentNetwork.AddressRange6
		hasholepunchupdate := newNetwork.DefaultUDPHolePunch != currentNetwork.DefaultUDPHolePunch
		newNetwork.SetNetworkLastModified()
		data, err := json.Marshal(newNetwork)
		if err != nil {
			return false, false, false, err
		}

		err = database.Insert(newNetwork.NetID, string(data), database.NETWORKS_TABLE_NAME)
		if err == nil {
			if servercfg.CacheEnabled() {
				storeNetworkInCache(newNetwork.NetID, *newNetwork)
			}
		}
		return hasrangeupdate4, hasrangeupdate6, hasholepunchupdate, err
	}
	// copy values
	return false, false, false, errors.New("failed to update network " + newNetwork.NetID + ", cannot change netid.")
}

func UpsertNetwork(net *models.Network) error {
	net.SetNetworkLastModified()
	data, err := json.Marshal(net)
	if err != nil {
		return err
	}

	err = database.Insert(net.NetID, string(data), database.NETWORKS_TABLE_NAME)
	if err == nil {
		if servercfg.CacheEnabled() {
			storeNetworkInCache(net.NetID, *net)
		}
	}
	return nil
}

// GetNetwork - gets a network from database
func GetNetwork(networkname string) (models.Network, error) {

	var network models.Network
	if servercfg.CacheEnabled() {
		if network, ok := getNetworkFromCache(networkname); ok {
			return network, nil
		}
	}
	networkData, err := database.FetchRecord(database.NETWORKS_TABLE_NAME, networkname)
	if err != nil {
		return network, err
	}
	if err = json.Unmarshal([]byte(networkData), &network); err != nil {
		return models.Network{}, err
	}
	return network, nil
}

// NetIDInNetworkCharSet - checks if a netid of a network uses valid characters
func NetIDInNetworkCharSet(network *models.Network) bool {

	charset := "abcdefghijklmnopqrstuvwxyz1234567890-_"

	for _, char := range network.NetID {
		if !strings.Contains(charset, string(char)) {
			return false
		}
	}
	return true
}

// Validate - validates fields of an network struct
func ValidateNetwork(network *models.Network, isUpdate bool) error {
	v := validator.New()
	_ = v.RegisterValidation("netid_valid", func(fl validator.FieldLevel) bool {
		inCharSet := NetIDInNetworkCharSet(network)
		if isUpdate {
			return inCharSet
		}
		isFieldUnique, _ := IsNetworkNameUnique(network)
		return isFieldUnique && inCharSet
	})
	//
	_ = v.RegisterValidation("checkyesorno", func(fl validator.FieldLevel) bool {
		return validation.CheckYesOrNo(fl)
	})
	err := v.Struct(network)
	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			fmt.Println(e)
		}
	}

	return err
}

// ParseNetwork - parses a network into a model
func ParseNetwork(value string) (models.Network, error) {
	var network models.Network
	err := json.Unmarshal([]byte(value), &network)
	return network, err
}

// SaveNetwork - save network struct to database
func SaveNetwork(network *models.Network) error {
	data, err := json.Marshal(network)
	if err != nil {
		return err
	}
	if err := database.Insert(network.NetID, string(data), database.NETWORKS_TABLE_NAME); err != nil {
		return err
	}
	if servercfg.CacheEnabled() {
		storeNetworkInCache(network.NetID, *network)
	}
	return nil
}

// NetworkExists - check if network exists
func NetworkExists(name string) (bool, error) {

	var network string
	var err error
	if servercfg.CacheEnabled() {
		if _, ok := getNetworkFromCache(name); ok {
			return ok, nil
		}
	}
	if network, err = database.FetchRecord(database.NETWORKS_TABLE_NAME, name); err != nil {
		return false, err
	}
	return len(network) > 0, nil
}

// SortNetworks - Sorts slice of Networks by their NetID alphabetically with numbers first
func SortNetworks(unsortedNetworks []models.Network) {
	sort.Slice(unsortedNetworks, func(i, j int) bool {
		return unsortedNetworks[i].NetID < unsortedNetworks[j].NetID
	})
}

// == Private ==

var addressLock = &sync.Mutex{}
