package logic

import (
	"context"
	"errors"
	"fmt"
	"github.com/gravitl/netmaker/converters"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic/acls/nodeacls"
	"github.com/gravitl/netmaker/schema"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/c-robinson/iplib"
	validator "github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/validation"
)

var (
	ErrNetworkNotIPv4 = errors.New("network is not ipv4")
	ErrNetworkNotIPv6 = errors.New("network is not ipv6")
	ErrNoIPAvailable  = errors.New("no ip available in the network")
)

// GetNetworks - returns all networks from database
func GetNetworks() ([]models.Network, error) {
	_networks, err := (&schema.Network{}).ListAll(db.WithContext(context.TODO()))
	if err != nil {
		return nil, err
	}

	return converters.ToModelNetworks(_networks), nil
}

// DeleteNetwork - deletes a network
func DeleteNetwork(network string, force bool, done chan struct{}) error {

	nodeCount, err := GetNetworkNonServerNodeCount(network)
	if nodeCount == 0 || database.IsEmptyRecord(err) {
		// delete server nodes first then db records
		_network := &schema.Network{
			ID: network,
		}
		return _network.Delete(db.WithContext(context.TODO()))
	}

	// Remove All Nodes
	go func() {
		nodes, err := GetNetworkNodes(network)
		if err == nil {
			for _, node := range nodes {
				node := node
				host, err := GetHost(node.HostID.String())
				if err != nil {
					continue
				}
				if node.IsGw {
					// delete ext clients belonging to gateway
					DeleteGatewayExtClients(node.ID.String(), node.Network)
				}
				DissasociateNodeFromHost(&node, host)
			}
		}

		// remove ACL for network
		err = nodeacls.DeleteACLContainer(nodeacls.NetworkID(network))
		if err != nil {
			logger.Log(1, "failed to remove the node acls during network delete for network,", network)
		}
		// delete server nodes first then db records
		_network := &schema.Network{
			ID: network,
		}
		err = _network.Delete(db.WithContext(context.TODO()))
		if err != nil {
			return
		}
		done <- struct{}{}
		close(done)
	}()

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

	return nil
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

	_network := converters.ToSchemaNetwork(network)
	err = _network.Create(db.WithContext(context.TODO()))
	if err != nil {
		return models.Network{}, err
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
		false,
	)

	return network, nil
}

// GetNetworkNonServerNodeCount - get number of network non server nodes
func GetNetworkNonServerNodeCount(networkName string) (int, error) {
	_network := &schema.Network{
		ID: networkName,
	}
	return _network.CountNodes(db.WithContext(context.TODO()))
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

// getAvailableIPv4Addr - get a unique ipv4 address
func getAvailableIPv4Addr(networkID string, reverse bool) (*net.IPNet, error) {
	_network := &schema.Network{
		ID: networkID,
	}
	err := _network.Get(db.WithContext(context.TODO()))
	if err != nil {
		return nil, err
	}

	if _network.IsIPv4 == "no" {
		return nil, ErrNetworkNotIPv4
	}

	_, ipv4Addr, err := net.ParseCIDR(_network.AddressRange)
	if err != nil {
		return nil, err
	}

	net4 := iplib.Net4FromStr(_network.AddressRange)
	ipv4Addr.IP = net4.FirstAddress()

	if reverse {
		ipv4Addr.IP = net4.LastAddress()
	}

	for {
		if nodeWithIPExists(networkID, ipv4Addr.String(), database.NODES_TABLE_NAME, false) &&
			nodeWithIPExists(networkID, ipv4Addr.String(), database.EXT_CLIENT_TABLE_NAME, false) {
			return ipv4Addr, nil
		}

		if reverse {
			ipv4Addr.IP, err = net4.PreviousIP(ipv4Addr.IP)
		} else {
			ipv4Addr.IP, err = net4.NextIP(ipv4Addr.IP)
		}
		if err != nil {
			break
		}
	}

	return nil, ErrNoIPAvailable
}

// getAvailableIPv6Addr - see if ipv6 address is unique
func getAvailableIPv6Addr(networkID string, reverse bool) (*net.IPNet, error) {
	_network := &schema.Network{
		ID: networkID,
	}
	err := _network.Get(db.WithContext(context.TODO()))
	if err != nil {
		return nil, err
	}

	if _network.IsIPv6 == "no" {
		return nil, ErrNetworkNotIPv6
	}

	_, ipv6Addr, err := net.ParseCIDR(_network.AddressRange6)
	if err != nil {
		return nil, err
	}

	net6 := iplib.Net6FromStr(_network.AddressRange6)
	ipv6Addr.IP, err = net6.NextIP(net6.FirstAddress())
	if reverse {
		ipv6Addr.IP, err = net6.PreviousIP(net6.LastAddress())
	}
	if err != nil {
		return nil, err
	}

	for {
		if nodeWithIPExists(networkID, ipv6Addr.String(), database.NODES_TABLE_NAME, true) &&
			nodeWithIPExists(networkID, ipv6Addr.String(), database.EXT_CLIENT_TABLE_NAME, true) {
			return ipv6Addr, nil
		}

		if reverse {
			ipv6Addr.IP, err = net6.PreviousIP(ipv6Addr.IP)
		} else {
			ipv6Addr.IP, err = net6.NextIP(ipv6Addr.IP)
		}
		if err != nil {
			break
		}
	}

	return nil, ErrNoIPAvailable
}

// nodeWithIPExists - checks if an IP is unique
func nodeWithIPExists(network string, ip string, tableName string, isIpv6 bool) bool {
	if tableName == database.NODES_TABLE_NAME {
		_node := &schema.Node{
			NetworkID: network,
		}

		if isIpv6 {
			_node.Address6 = ip
			exists, err := _node.ExistsWithNetworkAndIPv6(db.WithContext(context.TODO()))
			if err != nil {
				return true
			}

			// if a node exists, then the ip is not unique.
			return !exists
		} else {
			_node.Address = ip
			exists, err := _node.ExistsWithNetworkAndIPv4(db.WithContext(context.TODO()))
			if err != nil {
				return true
			}

			// if a node exists, then the ip is not unique.
			return !exists
		}
	} else if tableName == database.EXT_CLIENT_TABLE_NAME {

		extClients, err := GetNetworkExtClients(network)
		if err != nil {
			return true
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

	return true
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

		_network := converters.ToSchemaNetwork(*newNetwork)
		err := _network.Update(db.WithContext(context.TODO()))
		return hasrangeupdate4, hasrangeupdate6, hasholepunchupdate, err
	}
	// copy values
	return false, false, false, errors.New("failed to update network " + newNetwork.NetID + ", cannot change netid.")
}

// GetNetwork - gets a network from database
func GetNetwork(networkname string) (models.Network, error) {
	_network := &schema.Network{
		ID: networkname,
	}
	err := _network.Get(db.WithContext(context.TODO()))
	if err != nil {
		return models.Network{}, err
	}

	return converters.ToModelNetwork(*_network), nil
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

// SaveNetwork - save network struct to database
func SaveNetwork(network *models.Network) error {
	_network := converters.ToSchemaNetwork(*network)
	return _network.Update(db.WithContext(context.TODO()))
}

// NetworkExists - check if network exists
func NetworkExists(name string) (bool, error) {
	_network := &schema.Network{
		ID: name,
	}
	return _network.Exists(db.WithContext(context.TODO()))
}

// == Private ==

var addressLock = &sync.Mutex{}
