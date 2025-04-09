package logic

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/gravitl/netmaker/converters"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/exp/slog"
	"gorm.io/gorm"
	"sort"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

var (
	// ErrHostExists error indicating that host exists when trying to create new host
	ErrHostExists error = errors.New("host already exists")
	// ErrInvalidHostID error indicates that the id is nil and thus a host with that
	// id doesn't / cannot exist.
	ErrInvalidHostID error = errors.New("invalid host id")
)

const (
	maxPort = 1<<16 - 1
	minPort = 1025
)

// GetAllHosts - returns all hosts in flat list or error
func GetAllHosts() ([]models.Host, error) {
	_hosts, err := (&schema.Host{}).ListAll(db.WithContext(context.TODO()))
	if err != nil {
		return nil, err
	}

	var currHosts []models.Host
	for _, _host := range _hosts {
		host := converters.ToModelHost(_host)
		currHosts = append(currHosts, host)
	}

	return currHosts, nil
}

// GetHost - gets a host from db given id
func GetHost(hostID string) (*models.Host, error) {
	_host := &schema.Host{
		ID: hostID,
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
	numHosts, hErr := (&schema.Host{}).Count(db.WithContext(context.TODO()))
	clients, cErr := GetAllExtClients()
	if hErr != nil ||
		(cErr != nil && !database.IsEmptyRecord(cErr)) ||
		numHosts+len(clients) >= MachinesLimit {
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
	h.AutoUpdate = servercfg.AutoUpdateEnabled()
	checkForZombieHosts(h)
	return UpsertHost(h)
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
func UpsertHost(host *models.Host) error {
	_host := converters.ToSchemaHost(*host)
	return _host.Upsert(db.WithContext(context.TODO()))
}

// RemoveHost - removes a given host from server
func RemoveHost(h *models.Host, forceDelete bool) error {
	if !forceDelete && len(h.Nodes) > 0 {
		return fmt.Errorf("host still has associated nodes")
	}

	if len(h.Nodes) > 0 {
		if err := DisassociateAllNodesFromHost(h.ID.String()); err != nil {
			return err
		}
	}

	_host := &schema.Host{
		ID: h.ID.String(),
	}
	err := _host.Delete(db.WithContext(context.TODO()))
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

// UpdateHostNetwork - adds/deletes host from a network
func UpdateHostNetwork(h *models.Host, network string, add bool) (*models.Node, error) {
	for _, nodeID := range h.Nodes {
		node, err := GetNodeByID(nodeID)
		if err != nil || node.PendingDelete {
			continue
		}
		if node.Network == network {
			if !add {
				return &node, nil
			} else {
				return nil, errors.New("host already part of network " + network)
			}
		}
	}
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

// AssociateNodeToHost - associates and creates a node with a given host
// should be the only way nodes get created as of 0.18
func AssociateNodeToHost(n *models.Node, h *models.Host) error {
	if len(h.ID.String()) == 0 || h.ID == uuid.Nil {
		return ErrInvalidHostID
	}
	n.HostID = h.ID
	err := createNode(n)
	if err != nil {
		return err
	}
	currentHost, err := GetHost(h.ID.String())
	if err != nil {
		return err
	}
	h.HostPass = currentHost.HostPass
	h.Nodes = append(currentHost.Nodes, n.ID.String())
	return UpsertHost(h)
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
	if len(h.Nodes) == 0 {
		return fmt.Errorf("no nodes present in given host")
	}
	nList := []string{}
	for i := range h.Nodes {
		if h.Nodes[i] != n.ID.String() {
			nList = append(nList, h.Nodes[i])
		}
	}
	h.Nodes = nList
	go func() {
		if servercfg.IsPro {
			if clients, err := GetNetworkExtClients(n.Network); err != nil {
				for i := range clients {
					AllowClientNodeAccess(&clients[i], n.ID.String())
				}
			}
		}
	}()
	if err := DeleteNodeByID(n); err != nil {
		return err
	}
	return UpsertHost(h)
}

// DisassociateAllNodesFromHost - deletes all nodes of the host
func DisassociateAllNodesFromHost(hostID string) error {
	host, err := GetHost(hostID)
	if err != nil {
		return err
	}
	for _, nodeID := range host.Nodes {
		node, err := GetNodeByID(nodeID)
		if err != nil {
			logger.Log(0, "failed to get host node, node id:", nodeID, err.Error())
			continue
		}
		if err := DeleteNode(&node, true); err != nil {
			logger.Log(0, "failed to delete node", node.ID.String(), err.Error())
			continue
		}
		logger.Log(3, "deleted node", node.ID.String(), "of host", host.ID.String())
	}
	host.Nodes = []string{}
	return UpsertHost(host)
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

}

// CheckHostPorts checks host endpoints to ensures that hosts on the same server
// with the same endpoint have different listen ports
// in the case of 64535 hosts or more with same endpoint, ports will not be changed
func CheckHostPorts(h *models.Host) {
	portsInUse := make(map[int]bool, 0)
	hosts, err := GetAllHosts()
	if err != nil {
		return
	}
	for _, host := range hosts {
		if host.ID.String() == h.ID.String() {
			// skip self
			continue
		}
		if !host.EndpointIP.Equal(h.EndpointIP) {
			continue
		}
		portsInUse[host.ListenPort] = true
	}
	// iterate until port is not found or max iteration is reached
	for i := 0; portsInUse[h.ListenPort] && i < maxPort-minPort+1; i++ {
		h.ListenPort++
		if h.ListenPort > maxPort {
			h.ListenPort = minPort
		}
	}
}

// HostExists - checks if given host already exists
func HostExists(h *models.Host) bool {
	_, err := GetHost(h.ID.String())
	if err != nil {
		return false
	} else {
		return true
	}
}

// ConvHostPassToHash - converts password to md5 hash
func ConvHostPassToHash(hostPass string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(hostPass)))
}

// SortApiHosts - Sorts slice of ApiHosts by their ID alphabetically with numbers first
func SortApiHosts(unsortedHosts []models.ApiHost) {
	sort.Slice(unsortedHosts, func(i, j int) bool {
		return unsortedHosts[i].ID < unsortedHosts[j].ID
	})
}
