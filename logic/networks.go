package logic

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	nodeacls "github.com/gravitl/netmaker/logic/acls/node-acls"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
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
		servers, err := GetSortedNetworkServerNodes(network)
		if err == nil {
			for _, s := range servers {
				if err = DeleteNodeByID(&s, true); err != nil {
					logger.Log(2, "could not removed server", s.Name, "before deleting network", network)
				} else {
					logger.Log(2, "removed server", s.Name, "before deleting network", network)
				}
			}
		} else {
			logger.Log(1, "could not remove servers before deleting network", network)
		}
		return database.DeleteRecord(database.NETWORKS_TABLE_NAME, network)
	}
	return errors.New("node check failed. All nodes must be deleted before deleting network")
}

// CreateNetwork - creates a network in database
func CreateNetwork(network models.Network) error {

	network.SetDefaults()
	network.SetNodesLastModified()
	network.SetNetworkLastModified()

	err := ValidateNetwork(&network, false)
	if err != nil {
		//returnErrorResponse(w, r, formatError(err, "badrequest"))
		return err
	}

	data, err := json.Marshal(&network)
	if err != nil {
		return err
	}
	if err = database.Insert(network.NetID, string(data), database.NETWORKS_TABLE_NAME); err != nil {
		return err
	}

	return err
}

// NetworkNodesUpdatePullChanges - tells nodes on network to pull
func NetworkNodesUpdatePullChanges(networkName string) error {

	collections, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil {
		if database.IsEmptyRecord(err) {
			return nil
		}
		return err
	}

	for _, value := range collections {
		var node models.Node
		err := json.Unmarshal([]byte(value), &node)
		if err != nil {
			fmt.Println("error in node address assignment!")
			return err
		}
		if node.Network == networkName {
			data, err := json.Marshal(&node)
			if err != nil {
				return err
			}
			database.Insert(node.ID, string(data), database.NODES_TABLE_NAME)
		}
	}

	return nil
}

// GetNetworkNonServerNodeCount - get number of network non server nodes
func GetNetworkNonServerNodeCount(networkName string) (int, error) {

	collection, err := database.FetchRecords(database.NODES_TABLE_NAME)
	count := 0
	if err != nil && !database.IsEmptyRecord(err) {
		return count, err
	}
	for _, value := range collection {
		var node models.Node
		if err = json.Unmarshal([]byte(value), &node); err != nil {
			return count, err
		} else {
			if node.Network == networkName && node.IsServer != "yes" {
				count++
			}
		}
	}

	return count, nil
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
	network.AccessKeys = []models.AccessKey{}
	return network, nil
}

// UniqueAddress - see if address is unique
func UniqueAddress(networkName string) (string, error) {

	var network models.Network
	network, err := GetParentNetwork(networkName)
	if err != nil {
		fmt.Println("UniqueAddress encountered  an error")
		return "666", err
	}

	offset := true
	ip, ipnet, err := net.ParseCIDR(network.AddressRange)
	if err != nil {
		fmt.Println("UniqueAddress encountered  an error")
		return "666", err
	}
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); Inc(ip) {
		if offset {
			offset = false
			continue
		}
		if IsIPUnique(networkName, ip.String(), database.NODES_TABLE_NAME, false) && IsIPUnique(networkName, ip.String(), database.EXT_CLIENT_TABLE_NAME, false) {
			return ip.String(), err
		}
	}

	//TODO
	err1 := errors.New("ERROR: No unique addresses available. Check network subnet")
	return "W1R3: NO UNIQUE ADDRESSES AVAILABLE", err1
}

// UniqueAddressServer - get unique address starting from last available
func UniqueAddressServer(networkName string) (string, error) {

	var network models.Network
	network, err := GetParentNetwork(networkName)
	if err != nil {
		logger.Log(0, "UniqueAddressServer encountered  an error")
		return "666", err
	}

	_, ipv4Net, err := net.ParseCIDR(network.AddressRange)
	if err != nil {
		logger.Log(0, "UniqueAddressServer encountered  an error")
		return "666", err
	}

	// convert IPNet struct mask and address to uint32
	// network is BigEndian
	mask := binary.BigEndian.Uint32(ipv4Net.Mask)
	start := binary.BigEndian.Uint32(ipv4Net.IP)

	// find the final address
	finish := (start & mask) | (mask ^ 0xffffffff)

	// loop through addresses as uint32
	for i := finish - 1; i > start; i-- {
		// convert back to net.IP
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, i)
		if IsIPUnique(networkName, ip.String(), database.NODES_TABLE_NAME, false) && IsIPUnique(networkName, ip.String(), database.EXT_CLIENT_TABLE_NAME, false) {
			return ip.String(), err
		}
	}

	return "W1R3: NO UNIQUE ADDRESSES AVAILABLE", fmt.Errorf("no unique server addresses found")
}

// IsIPUnique - checks if an IP is unique
func IsIPUnique(network string, ip string, tableName string, isIpv6 bool) bool {

	isunique := true
	collection, err := database.FetchRecords(tableName)

	if err != nil {
		return isunique
	}

	for _, value := range collection { // filter
		var node models.Node
		if err = json.Unmarshal([]byte(value), &node); err != nil {
			continue
		}
		if isIpv6 {
			if node.Address6 == ip && node.Network == network {
				return false
			}
		} else {
			if node.Address == ip && node.Network == network {
				return false
			}
		}
	}

	return isunique
}

// UniqueAddress6 - see if ipv6 address is unique
func UniqueAddress6(networkName string) (string, error) {

	var network models.Network
	network, err := GetParentNetwork(networkName)
	if err != nil {
		fmt.Println("Network Not Found")
		return "", err
	}
	if network.IsDualStack == "no" {
		return "", nil
	}

	offset := true
	ip, ipnet, err := net.ParseCIDR(network.AddressRange6)
	if err != nil {
		fmt.Println("UniqueAddress6 encountered  an error")
		return "666", err
	}
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); Inc(ip) {
		if offset {
			offset = false
			continue
		}
		if IsIPUnique(networkName, ip.String(), database.NODES_TABLE_NAME, true) {
			return ip.String(), err
		}
	}
	//TODO
	err1 := errors.New("ERROR: No unique addresses available. Check network subnet")
	return "W1R3: NO UNIQUE ADDRESSES AVAILABLE", err1
}

// GetLocalIP - gets the local ip
func GetLocalIP(node models.Node) string {

	var local string

	ifaces, err := net.Interfaces()
	if err != nil {
		return local
	}
	_, localrange, err := net.ParseCIDR(node.LocalRange)
	if err != nil {
		return local
	}

	found := false
	for _, i := range ifaces {
		if i.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if i.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := i.Addrs()
		if err != nil {
			return local
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				if !found {
					ip = v.IP
					local = ip.String()
					if node.IsLocal == "yes" {
						found = localrange.Contains(ip)
					} else {
						found = true
					}
				}
			case *net.IPAddr:
				if !found {
					ip = v.IP
					local = ip.String()
					if node.IsLocal == "yes" {
						found = localrange.Contains(ip)

					} else {
						found = true
					}
				}
			}
		}
	}
	return local
}

// UpdateNetworkLocalAddresses - updates network localaddresses
func UpdateNetworkLocalAddresses(networkName string) error {

	collection, err := database.FetchRecords(database.NODES_TABLE_NAME)

	if err != nil {
		return err
	}

	for _, value := range collection {

		var node models.Node

		err := json.Unmarshal([]byte(value), &node)
		if err != nil {
			fmt.Println("error in node address assignment!")
			return err
		}
		if node.Network == networkName {
			var ipaddr string
			var iperr error
			if node.IsServer == "yes" {
				ipaddr, iperr = UniqueAddressServer(networkName)
			} else {
				ipaddr, iperr = UniqueAddress(networkName)
			}
			if iperr != nil {
				fmt.Println("error in node  address assignment!")
				return iperr
			}

			node.Address = ipaddr
			newNodeData, err := json.Marshal(&node)
			if err != nil {
				logger.Log(1, "error in node  address assignment!")
				return err
			}
			database.Insert(node.ID, string(newNodeData), database.NODES_TABLE_NAME)
		}
	}

	return nil
}

// UpdateNetworkLocalAddresses - updates network localaddresses
func UpdateNetworkHolePunching(networkName string, holepunch string) error {

	nodes, err := GetNetworkNodes(networkName)
	if err != nil {
		return err
	}

	for _, node := range nodes {
		if node.IsServer != "yes" {
			node.UDPHolePunch = holepunch
			newNodeData, err := json.Marshal(&node)
			if err != nil {
				logger.Log(1, "error in node hole punch assignment")
				return err
			}
			database.Insert(node.ID, string(newNodeData), database.NODES_TABLE_NAME)
		}
	}
	return nil
}

// RemoveNetworkNodeIPv6Addresses - removes network node IPv6 addresses
func RemoveNetworkNodeIPv6Addresses(networkName string) error {

	collections, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil {
		return err
	}

	for _, value := range collections {

		var node models.Node
		err := json.Unmarshal([]byte(value), &node)
		if err != nil {
			fmt.Println("error in node address assignment!")
			return err
		}
		if node.Network == networkName {
			node.IsDualStack = "no"
			node.Address6 = ""
			data, err := json.Marshal(&node)
			if err != nil {
				return err
			}
			database.Insert(node.ID, string(data), database.NODES_TABLE_NAME)
		}
	}

	return nil
}

// UpdateNetworkNodeAddresses - updates network node addresses
func UpdateNetworkNodeAddresses(networkName string) error {

	collections, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil {
		return err
	}

	for _, value := range collections {

		var node models.Node
		err := json.Unmarshal([]byte(value), &node)
		if err != nil {
			fmt.Println("error in node address assignment!")
			return err
		}
		if node.Network == networkName {
			var ipaddr string
			var iperr error
			if node.IsServer == "yes" {
				ipaddr, iperr = UniqueAddressServer(networkName)
			} else {
				ipaddr, iperr = UniqueAddress(networkName)
			}
			if iperr != nil {
				fmt.Println("error in node  address assignment!")
				return iperr
			}

			node.Address = ipaddr
			data, err := json.Marshal(&node)
			if err != nil {
				return err
			}
			database.Insert(node.ID, string(data), database.NODES_TABLE_NAME)
		}
	}

	return nil
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
		hasrangeupdate := newNetwork.AddressRange != currentNetwork.AddressRange
		localrangeupdate := newNetwork.LocalRange != currentNetwork.LocalRange
		hasholepunchupdate := newNetwork.DefaultUDPHolePunch != currentNetwork.DefaultUDPHolePunch
		data, err := json.Marshal(newNetwork)
		if err != nil {
			return false, false, false, err
		}
		newNetwork.SetNetworkLastModified()
		err = database.Insert(newNetwork.NetID, string(data), database.NETWORKS_TABLE_NAME)
		return hasrangeupdate, localrangeupdate, hasholepunchupdate, err
	}
	// copy values
	return false, false, false, errors.New("failed to update network " + newNetwork.NetID + ", cannot change netid.")
}

// Inc - increments an IP
func Inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
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

// Network.NetIDInNetworkCharSet - checks if a netid of a network uses valid characters
func NetIDInNetworkCharSet(network *models.Network) bool {

	charset := "abcdefghijklmnopqrstuvwxyz1234567890-_."

	for _, char := range network.NetID {
		if !strings.Contains(charset, strings.ToLower(string(char))) {
			return false
		}
	}
	return true
}

// Network.Validate - validates fields of an network struct
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

// ValidateNetworkUpdate - checks if network is valid to update
func ValidateNetworkUpdate(network models.Network) error {
	v := validator.New()

	_ = v.RegisterValidation("netid_valid", func(fl validator.FieldLevel) bool {
		if fl.Field().String() == "" {
			return true
		}
		inCharSet := nameInNetworkCharSet(fl.Field().String())
		return inCharSet
	})

	err := v.Struct(network)

	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			logger.Log(1, "validator", e.Error())
		}
	}
	return err
}

// KeyUpdate - updates keys on network
func KeyUpdate(netname string) (models.Network, error) {
	err := networkNodesUpdateAction(netname, models.NODE_UPDATE_KEY)
	if err != nil {
		return models.Network{}, err
	}
	return models.Network{}, nil
}

//SaveNetwork - save network struct to database
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

// == Private ==

func networkNodesUpdateAction(networkName string, action string) error {

	collections, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil {
		if database.IsEmptyRecord(err) {
			return nil
		}
		return err
	}

	for _, value := range collections {
		var node models.Node
		err := json.Unmarshal([]byte(value), &node)
		if err != nil {
			fmt.Println("error in node address assignment!")
			return err
		}
		if action == models.NODE_UPDATE_KEY && node.IsStatic == "yes" {
			continue
		}
		if node.Network == networkName {
			node.Action = action
			data, err := json.Marshal(&node)
			if err != nil {
				return err
			}
			database.Insert(node.ID, string(data), database.NODES_TABLE_NAME)
		}
	}
	return nil
}

func nameInNetworkCharSet(name string) bool {

	charset := "abcdefghijklmnopqrstuvwxyz1234567890-_."

	for _, char := range name {
		if !strings.Contains(charset, strings.ToLower(string(char))) {
			return false
		}
	}
	return true
}

func deleteInterface(ifacename string, postdown string) error {
	var err error
	if !ncutils.IsKernel() {
		err = RemoveConf(ifacename, true)
	} else {
		ipExec, errN := exec.LookPath("ip")
		err = errN
		if err != nil {
			logger.Log(1, err.Error())
		}
		_, err = ncutils.RunCmd(ipExec+" link del "+ifacename, false)
		if postdown != "" {
			runcmds := strings.Split(postdown, "; ")
			err = ncutils.RunCmds(runcmds, false)
		}
	}
	return err
}

func isInterfacePresent(iface string, address string) (string, bool) {
	var interfaces []net.Interface
	var err error
	interfaces, err = net.Interfaces()
	if err != nil {
		logger.Log(0, "ERROR: could not read interfaces")
		return "", true
	}
	for _, currIface := range interfaces {
		var currAddrs []net.Addr
		currAddrs, err = currIface.Addrs()
		if err != nil || len(currAddrs) == 0 {
			continue
		}
		for _, addr := range currAddrs {
			if strings.Contains(addr.String(), address) && currIface.Name != iface {
				// logger.Log(2, "found iface", addr.String(), currIface.Name)
				interfaces = nil
				currAddrs = nil
				return currIface.Name, false
			}
		}
		currAddrs = nil
	}
	interfaces = nil
	// logger.Log(2, "failed to find iface", iface)
	return "", true
}
