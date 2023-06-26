package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/c-robinson/iplib"
	validator "github.com/go-playground/validator/v10"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic/acls/nodeacls"
	"github.com/gravitl/netmaker/logic/pro"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/validation"
)

// GetNetworks - returns all networks from database
func GetNetworks() ([]models.Network, error) {
	var networks []models.Network

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
	nodeCount, err := GetNetworkNonServerNodeCount(network)
	if nodeCount == 0 || database.IsEmptyRecord(err) {
		// delete server nodes first then db records
		if err = pro.RemoveAllNetworkUsers(network); err != nil {
			logger.Log(0, "failed to remove network users on network delete for network", network, err.Error())
		}
		return database.DeleteRecord(database.NETWORKS_TABLE_NAME, network)
	}
	return errors.New("node check failed. All nodes must be deleted before deleting network")
}

// CreateNetwork - creates a network in database
func CreateNetwork(network models.Network) (models.Network, error) {

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

	network.SetDefaults()
	network.SetNodesLastModified()
	network.SetNetworkLastModified()

	pro.AddProNetDefaults(&network)

	if len(network.ProSettings.AllowedGroups) == 0 {
		network.ProSettings.AllowedGroups = []string{pro.DEFAULT_ALLOWED_GROUPS}
	}

	err := ValidateNetwork(&network, false)
	if err != nil {
		//logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return models.Network{}, err
	}

	if err = pro.InitializeNetworkUsers(network.NetID); err != nil {
		return models.Network{}, err
	}

	data, err := json.Marshal(&network)
	if err != nil {
		return models.Network{}, err
	}

	if err = database.Insert(network.NetID, string(data), database.NETWORKS_TABLE_NAME); err != nil {
		return models.Network{}, err
	}

	// == add all current users to network as network users ==
	if err = InitializeNetUsers(&network); err != nil {
		return network, err
	}

	return network, nil
}

// GetNetworkNonServerNodeCount - get number of network non server nodes
func GetNetworkNonServerNodeCount(networkName string) (int, error) {
	nodes, err := GetNetworkNodes(networkName)
	return len(nodes), err
}

// GetParentNetwork - get parent network
func GetParentNetwork(networkname string) (models.Network, error) {

	var network models.Network
	networkData, err := database.FetchRecord(database.NETWORKS_TABLE_NAME, networkname)
	if err != nil {
		return network, err
	}
	if err = json.Unmarshal([]byte(networkData), &network); err != nil {
		return models.Network{}, err
	}
	return network, nil
}

// GetParentNetwork - get parent network
func GetNetworkSettings(networkname string) (models.Network, error) {

	var network models.Network
	networkData, err := database.FetchRecord(database.NETWORKS_TABLE_NAME, networkname)
	if err != nil {
		return network, err
	}
	if err = json.Unmarshal([]byte(networkData), &network); err != nil {
		return models.Network{}, err
	}
	return network, nil
}

// UniqueAddress - see if address is unique
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

	for {
		if IsIPUnique(networkName, newAddrs.String(), database.NODES_TABLE_NAME, false) &&
			IsIPUnique(networkName, newAddrs.String(), database.EXT_CLIENT_TABLE_NAME, false) {
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
		collection, err := database.FetchRecords(tableName)
		if err != nil {
			return isunique
		}
		var extClient models.ExtClient
		for _, value := range collection { // filter
			if err = json.Unmarshal([]byte(value), &extClient); err != nil {
				continue
			}
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

	for {
		if IsIPUnique(networkName, newAddrs.String(), database.NODES_TABLE_NAME, true) &&
			IsIPUnique(networkName, newAddrs.String(), database.EXT_CLIENT_TABLE_NAME, true) {
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
func UpdateNetwork(currentNetwork *models.Network, newNetwork *models.Network) (bool, bool, bool, []string, []string, error) {
	if err := ValidateNetwork(newNetwork, true); err != nil {
		return false, false, false, nil, nil, err
	}
	if newNetwork.NetID == currentNetwork.NetID {
		hasrangeupdate4 := newNetwork.AddressRange != currentNetwork.AddressRange
		hasrangeupdate6 := newNetwork.AddressRange6 != currentNetwork.AddressRange6
		hasholepunchupdate := newNetwork.DefaultUDPHolePunch != currentNetwork.DefaultUDPHolePunch
		groupDelta := append(StringDifference(newNetwork.ProSettings.AllowedGroups, currentNetwork.ProSettings.AllowedGroups),
			StringDifference(currentNetwork.ProSettings.AllowedGroups, newNetwork.ProSettings.AllowedGroups)...)
		userDelta := append(StringDifference(newNetwork.ProSettings.AllowedUsers, currentNetwork.ProSettings.AllowedUsers),
			StringDifference(currentNetwork.ProSettings.AllowedUsers, newNetwork.ProSettings.AllowedUsers)...)
		data, err := json.Marshal(newNetwork)
		if err != nil {
			return false, false, false, nil, nil, err
		}
		newNetwork.SetNetworkLastModified()
		err = database.Insert(newNetwork.NetID, string(data), database.NETWORKS_TABLE_NAME)
		return hasrangeupdate4, hasrangeupdate6, hasholepunchupdate, groupDelta, userDelta, err
	}
	// copy values
	return false, false, false, nil, nil, errors.New("failed to update network " + newNetwork.NetID + ", cannot change netid.")
}

// GetNetwork - gets a network from database
func GetNetwork(networkname string) (models.Network, error) {

	var network models.Network
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

	if network.ProSettings != nil {
		if network.ProSettings.DefaultAccessLevel < pro.NET_ADMIN || network.ProSettings.DefaultAccessLevel > pro.NO_ACCESS {
			return fmt.Errorf("invalid access level")
		}
		if network.ProSettings.DefaultUserClientLimit < 0 || network.ProSettings.DefaultUserNodeLimit < 0 {
			return fmt.Errorf("invalid node/client limit provided")
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
	return nil
}

// NetworkExists - check if network exists
func NetworkExists(name string) (bool, error) {

	var network string
	var err error
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
