package logic

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/converters"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/exp/slog"
	"gorm.io/gorm"
)

var (
	// ErrHostExists error indicating that host exists when trying to create new host
	ErrHostExists error = errors.New("host already exists")
	// ErrInvalidHostID
	ErrInvalidHostID error = errors.New("invalid host id")
)

const (
	maxPort = 1<<16 - 1
	minPort = 1025
)

var GetHostLocInfo = func(ip, token string) string { return "" }

// GetAllHosts - returns all hosts in flat list or error
func GetAllHosts() ([]models.Host, error) {
	_hosts, err := (&schema.Host{}).ListAll(db.WithContext(context.TODO()))
	if err != nil {
		return nil, err
	}

	hosts := converters.ToModelHosts(_hosts)
	for i := range hosts {
		// TODO: OPTIMIZE MEEEE!!
		// TODO: remove this and update every
		// TODO: code expecting nodes to query.
		_host := &schema.Host{
			ID: hosts[i].ID.String(),
		}
		_hostNodes, _ := _host.GetNodes(db.WithContext(context.TODO()))

		nodes := make([]string, len(_hostNodes))
		for i, _node := range _hostNodes {
			nodes[i] = _node.ID
		}

		hosts[i].Nodes = nodes
	}

	return hosts, nil
}

// GetAllHostsWithStatus - returns all hosts with at least one
// node with given status.
func GetAllHostsWithStatus(status models.NodeStatus) ([]models.Host, error) {
	hosts, err := GetAllHosts()
	if err != nil {
		return nil, err
	}

	var validHosts []models.Host
	for _, host := range hosts {
		if len(host.Nodes) == 0 {
			continue
		}

		nodes := GetHostNodes(&host)
		for _, node := range nodes {
			GetNodeCheckInStatus(&node, false)
			if node.Status == status {
				validHosts = append(validHosts, host)
				break
			}
		}
	}

	return validHosts, nil
}

// GetAllHostsAPI - get's all the hosts in an API usable format
func GetAllHostsAPI(hosts []models.Host) []models.ApiHost {
	apiHosts := []models.ApiHost{}
	for i := range hosts {
		newApiHost := hosts[i].ConvertNMHostToAPI()
		apiHosts = append(apiHosts, *newApiHost)
	}
	return apiHosts[:]
}

// GetHost - gets a host from db given id
func GetHost(hostid string) (*models.Host, error) {
	_host := &schema.Host{
		ID: hostid,
	}
	err := _host.Get(db.WithContext(context.TODO()))
	if err != nil {
		return nil, err
	}

	host := converters.ToModelHost(*_host)
	return &host, nil
}

// CreateHost - creates a host if not exist
func CreateHost(h *models.Host) error {
	hosts, hErr := GetAllHosts()
	clients, cErr := GetAllExtClients()
	if (hErr != nil && !database.IsEmptyRecord(hErr)) ||
		(cErr != nil && !database.IsEmptyRecord(cErr)) ||
		len(hosts)+len(clients) >= MachinesLimit {
		return errors.New("free tier limits exceeded on machines")
	}
	_, err := GetHost(h.ID.String())
	if (err != nil && !errors.Is(err, gorm.ErrRecordNotFound)) || (err == nil) {
		return ErrHostExists
	}

	// encrypt that password so we never see it
	hash, err := bcrypt.GenerateFromPassword([]byte(h.HostPass), 5)
	if err != nil {
		return err
	}
	h.HostPass = string(hash)
	h.AutoUpdate = AutoUpdateEnabled()

	if GetServerSettings().ManageDNS {
		h.DNS = "yes"
	} else {
		h.DNS = "no"
	}
	if h.EndpointIP != nil {
		h.Location = GetHostLocInfo(h.EndpointIP.String(), os.Getenv("IP_INFO_TOKEN"))
	} else if h.EndpointIPv6 != nil {
		h.Location = GetHostLocInfo(h.EndpointIPv6.String(), os.Getenv("IP_INFO_TOKEN"))
	}
	checkForZombieHosts(h)
	return UpsertHost(h)
}

// UpdateHost - updates host data by field
func UpdateHost(newHost, currentHost *models.Host) {
	// unchangeable fields via API here
	newHost.DaemonInstalled = currentHost.DaemonInstalled
	newHost.OS = currentHost.OS
	newHost.IPForwarding = currentHost.IPForwarding
	newHost.HostPass = currentHost.HostPass
	newHost.MacAddress = currentHost.MacAddress
	newHost.Debug = currentHost.Debug
	newHost.Nodes = currentHost.Nodes
	newHost.PublicKey = currentHost.PublicKey
	newHost.TrafficKeyPublic = currentHost.TrafficKeyPublic
	// changeable fields
	if len(newHost.Version) == 0 {
		newHost.Version = currentHost.Version
	}

	if len(newHost.Name) == 0 {
		newHost.Name = currentHost.Name
	}

	if newHost.MTU == 0 {
		newHost.MTU = currentHost.MTU
	}

	if newHost.ListenPort == 0 {
		newHost.ListenPort = currentHost.ListenPort
	}

	if newHost.PersistentKeepalive == 0 {
		newHost.PersistentKeepalive = currentHost.PersistentKeepalive
	}
}

// UpdateHostFromClient - used for updating host on server with update recieved from client
func UpdateHostFromClient(newHost, currHost *models.Host) (sendPeerUpdate bool) {
	if newHost.PublicKey != currHost.PublicKey {
		currHost.PublicKey = newHost.PublicKey
		sendPeerUpdate = true
	}
	if newHost.ListenPort != 0 && currHost.ListenPort != newHost.ListenPort {
		currHost.ListenPort = newHost.ListenPort
		sendPeerUpdate = true
	}
	if newHost.WgPublicListenPort != 0 &&
		currHost.WgPublicListenPort != newHost.WgPublicListenPort {
		currHost.WgPublicListenPort = newHost.WgPublicListenPort
		sendPeerUpdate = true
	}
	isEndpointChanged := false
	if currHost.EndpointIP.String() != newHost.EndpointIP.String() {
		currHost.EndpointIP = newHost.EndpointIP
		sendPeerUpdate = true
		isEndpointChanged = true
	}
	if currHost.EndpointIPv6.String() != newHost.EndpointIPv6.String() {
		currHost.EndpointIPv6 = newHost.EndpointIPv6
		sendPeerUpdate = true
		isEndpointChanged = true
	}
	if !reflect.DeepEqual(currHost.Interfaces, newHost.Interfaces) {
		currHost.Interfaces = newHost.Interfaces
		sendPeerUpdate = true
	}

	if isEndpointChanged {
		for _, nodeID := range currHost.Nodes {
			node, err := GetNodeByID(nodeID)
			if err != nil {
				slog.Error("failed to get node:", "id", node.ID, "error", err)
				continue
			}
			if node.FailedOverBy != uuid.Nil {
				ResetFailedOverPeer(&node)
			}
		}
	}

	currHost.DaemonInstalled = newHost.DaemonInstalled
	currHost.Debug = newHost.Debug
	currHost.Verbosity = newHost.Verbosity
	currHost.Version = newHost.Version
	currHost.IsStaticPort = newHost.IsStaticPort
	currHost.IsStatic = newHost.IsStatic
	currHost.MTU = newHost.MTU

	currHost.Name = newHost.Name
	if len(newHost.NatType) > 0 && newHost.NatType != currHost.NatType {
		currHost.NatType = newHost.NatType
		sendPeerUpdate = true
	}

	return
}

// UpsertHost - upserts into DB a given host model, does not check for existence*
func UpsertHost(h *models.Host) error {
	_host := converters.ToSchemaHost(*h)
	return _host.Upsert(db.WithContext(context.TODO()))
}

// RemoveHost - removes a given host from server
func RemoveHost(h *models.Host, forceDelete bool) error {
	_host := &schema.Host{
		ID: h.ID.String(),
	}
	nodeCount, err := _host.CountNodes(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}

	if !forceDelete && nodeCount > 0 {
		return fmt.Errorf("host still has associated nodes")
	}

	if nodeCount > 0 {
		if err := DisassociateAllNodesFromHost(h.ID.String()); err != nil {
			return err
		}
	}

	err = RemoveHostByID(h.ID.String())
	if err != nil {
		return err
	}
	go func() {
		if servercfg.IsDNSMode() {
			SetDNS()
		}
	}()

	return nil
}

// RemoveHostByID - removes a given host by id from server
func RemoveHostByID(hostID string) error {
	_host := &schema.Host{
		ID: hostID,
	}
	return _host.Delete(db.WithContext(context.TODO()))
}

// UpdateHostNetwork - adds/deletes host from a network
func UpdateHostNetwork(h *models.Host, network string, add bool) (*models.Node, error) {
	_node := &schema.Node{
		HostID:    h.ID.String(),
		NetworkID: network,
	}
	err := _node.GetByHostIDAndNetworkID(db.WithContext(context.TODO()))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if !add {
				return nil, errors.New("host not part of the network " + network)
			} else {
				newNode := models.Node{}
				newNode.Server = servercfg.GetServer()
				newNode.Network = network
				newNode.HostID = h.ID
				if err := AssociateNodeToHost(&newNode, h); err != nil {
					return nil, err
				}
				return &newNode, nil
			}
		}
		return nil, err
	} else {
		if !add {
			node := converters.ToModelNode(*_node)
			return &node, nil
		} else {
			return nil, errors.New("host already part of network " + network)
		}
	}
}

// AssociateNodeToHost - associates and creates a node with a given host
// should be the only way nodes get created as of 0.18
func AssociateNodeToHost(n *models.Node, h *models.Host) error {
	if len(h.ID.String()) == 0 || h.ID == uuid.Nil {
		return ErrInvalidHostID
	}
	n.HostID = h.ID
	return createNode(n)
}

// DissasociateNodeFromHost - deletes a node and removes from host nodes
// should be the only way nodes are deleted as of 0.18
func DissasociateNodeFromHost(n *models.Node, h *models.Host) error {
	if len(h.ID.String()) == 0 || h.ID == uuid.Nil {
		return ErrInvalidHostID
	}
	if n.HostID != h.ID { // check if node actually belongs to host
		return fmt.Errorf("node is not associated with host")
	}

	go func() {
		if servercfg.IsPro {
			if clients, err := GetNetworkExtClients(n.Network); err != nil {
				for i := range clients {
					AllowClientNodeAccess(&clients[i], n.ID.String())
				}
			}
		}
	}()

	return DeleteNodeByID(n)
}

// DisassociateAllNodesFromHost - deletes all nodes of the host
func DisassociateAllNodesFromHost(hostID string) error {
	_host := &schema.Host{
		ID: hostID,
	}
	_hostNodes, err := _host.GetNodes(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}

	hostNodes := converters.ToModelNodes(_hostNodes)

	for _, node := range hostNodes {
		if err := DeleteNode(&node, true); err != nil {
			logger.Log(0, "failed to delete node", node.ID.String(), err.Error())
			continue
		}
		logger.Log(3, "deleted node", node.ID.String(), "of host", hostID)
	}

	return nil
}

// GetDefaultHosts - retrieve all hosts marked as default from DB
func GetDefaultHosts() []models.Host {
	defaultHostList := []models.Host{}
	hosts, err := GetAllHosts()
	if err != nil {
		return defaultHostList
	}
	for i := range hosts {
		if hosts[i].IsDefault {
			defaultHostList = append(defaultHostList, hosts[i])
		}
	}
	return defaultHostList[:]
}

// GetHostNetworks - fetches all the networks
func GetHostNetworks(hostID string) []string {
	currHost, err := GetHost(hostID)
	if err != nil {
		return nil
	}
	nets := []string{}
	for i := range currHost.Nodes {
		n, err := GetNodeByID(currHost.Nodes[i])
		if err != nil {
			return nil
		}
		nets = append(nets, n.Network)
	}
	return nets
}

// CheckHostPort checks host endpoints to ensures that hosts on the same server
// with the same endpoint have different listen ports
// in the case of 64535 hosts or more with same endpoint, ports will not be changed
func CheckHostPorts(h *models.Host) (changed bool) {
	portsInUse := make(map[int]bool, 0)
	hosts, err := GetAllHosts()
	if err != nil {
		return
	}
	originalPort := h.ListenPort
	defer func() {
		if originalPort != h.ListenPort {
			changed = true
		}
	}()
	if h.EndpointIP == nil {
		return
	}
	for _, host := range hosts {
		if host.ID.String() == h.ID.String() {
			// skip self
			continue
		}
		if host.EndpointIP == nil {
			continue
		}
		if !host.EndpointIP.Equal(h.EndpointIP) {
			continue
		}
		portsInUse[host.ListenPort] = true
	}
	// iterate until port is not found or max iteration is reached
	for i := 0; portsInUse[h.ListenPort] && i < maxPort-minPort+1; i++ {
		if h.ListenPort == 443 {
			h.ListenPort = 51821
		} else {
			h.ListenPort++
		}
		if h.ListenPort > maxPort {
			h.ListenPort = minPort
		}
	}
	return
}

// HostExists - checks if given host already exists
func HostExists(h *models.Host) bool {
	_, err := GetHost(h.ID.String())
	return (err != nil && !errors.Is(err, gorm.ErrRecordNotFound)) || (err == nil)
}
