package logic

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/exp/slog"
	"gorm.io/gorm"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/utils"
)

var (
	hostPortMutex = &sync.Mutex{}
)

var (
	// ErrHostExists error indicating that host exists when trying to create new host
	ErrHostExists error = errors.New("host already exists")
	// ErrInvalidHostID
	ErrInvalidHostID error = errors.New("invalid host id")
)

var CheckPostureViolations = func(d models.PostureCheckDeviceInfo, network schema.NetworkID) (v []models.Violation, level schema.Severity) {
	return []models.Violation{}, schema.SeverityUnknown
}

var GetPostureCheckDeviceInfoByNode = func(node *models.Node) (d models.PostureCheckDeviceInfo) {
	return
}

const (
	maxPort = 1<<16 - 1
	minPort = 1025
)

// GetAllHostsWithStatus - returns all hosts with at least one
// node with given status.
func GetAllHostsWithStatus(status models.NodeStatus) ([]schema.Host, error) {
	hosts, err := (&schema.Host{}).ListAll(db.WithContext(context.TODO()))
	if err != nil {
		return nil, err
	}

	var validHosts []schema.Host
	for _, host := range hosts {
		if len(host.Nodes) == 0 {
			continue
		}

		nodes := GetHostNodes(&host)
		for _, node := range nodes {
			getNodeCheckInStatus(&node, false)
			if node.Status == status {
				validHosts = append(validHosts, host)
				break
			}
		}
	}

	return validHosts, nil
}

// GetAllHostsAPI - get's all the hosts in an API usable format
func GetAllHostsAPI(hosts []schema.Host) []models.ApiHost {
	apiHosts := []models.ApiHost{}
	for i := range hosts {
		newApiHost := models.NewApiHostFromSchemaHost(&hosts[i])
		apiHosts = append(apiHosts, *newApiHost)
	}
	return apiHosts[:]
}

func DoesHostExistinTheNetworkAlready(h *schema.Host, network schema.NetworkID) bool {
	if len(h.Nodes) > 0 {
		for _, nodeID := range h.Nodes {
			node, err := GetNodeByID(nodeID)
			if err == nil && node.Network == network.String() {
				return true
			}
		}
	}
	return false
}

// CreateHost - creates a host if not exist
func CreateHost(h *schema.Host) error {
	_host := &schema.Host{ID: h.ID}
	err := _host.Get(db.WithContext(context.TODO()))
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

	if !GetFeatureFlags().EnableFlowLogs || !GetServerSettings().EnableFlowLogs {
		h.EnableFlowLogs = false
	}

	checkForZombieHosts(h)
	return UpsertHost(h)
}

// UpdateHost - updates host data by field
func UpdateHost(newHost, currentHost *schema.Host) {
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

	if strings.TrimSpace(newHost.DNS) == "" {
		newHost.DNS = currentHost.DNS
	}

	if !GetFeatureFlags().EnableFlowLogs || !GetServerSettings().EnableFlowLogs {
		newHost.EnableFlowLogs = false
	}
}

// UpdateHostFromClient - used for updating host on server with update recieved from client
func UpdateHostFromClient(newHost, currHost *schema.Host) (sendPeerUpdate bool) {
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
	if !currHost.EndpointIP.Equal(newHost.EndpointIP) {
		currHost.EndpointIP = newHost.EndpointIP
		sendPeerUpdate = true
		isEndpointChanged = true
	}
	if !currHost.EndpointIPv6.Equal(newHost.EndpointIPv6) {
		currHost.EndpointIPv6 = newHost.EndpointIPv6
		sendPeerUpdate = true
		isEndpointChanged = true
	}
	for i := range newHost.Interfaces {
		newHost.Interfaces[i].AddressString = newHost.Interfaces[i].Address.String()
	}
	utils.SortIfacesByName(currHost.Interfaces)
	utils.SortIfacesByName(newHost.Interfaces)
	if !utils.CompareIfaces(currHost.Interfaces, newHost.Interfaces) {
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
			if len(node.AutoRelayedPeers) > 0 {
				ResetAutoRelayedPeer(&node)
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
	if newHost.Location != "" {
		currHost.Location = newHost.Location
	}
	if newHost.CountryCode != "" {
		currHost.CountryCode = newHost.CountryCode
	}
	if isEndpointChanged || currHost.Location == "" || currHost.CountryCode == "" {
		var nodeIP net.IP
		if currHost.EndpointIP != nil {
			nodeIP = currHost.EndpointIP
		} else if currHost.EndpointIPv6 != nil {
			nodeIP = currHost.EndpointIPv6
		}

		if nodeIP != nil {
			info, err := utils.GetGeoInfo(nodeIP)
			if err == nil {
				currHost.Location = info.Location
				currHost.CountryCode = info.CountryCode
			}
		}
	}
	currHost.Name = newHost.Name
	if len(newHost.NatType) > 0 && newHost.NatType != currHost.NatType {
		currHost.NatType = newHost.NatType
		sendPeerUpdate = true
	}

	return
}

// UpsertHost - upserts into DB a given host model, does not check for existence*
func UpsertHost(h *schema.Host) error {
	return h.Upsert(db.WithContext(context.TODO()))
}

// UpdateHostNode -  handles updates from client nodes
func UpdateHostNode(h *schema.Host, newNode *models.Node) (publishDeletedNodeUpdate, publishPeerUpdate bool) {
	currentNode, err := GetNodeByID(newNode.ID.String())
	if err != nil {
		return
	}
	ifaceDelta := IfaceDelta(&currentNode, newNode)
	newNode.SetLastCheckIn()
	if err := UpdateNode(&currentNode, newNode); err != nil {
		slog.Error("error saving node", "name", h.Name, "network", newNode.Network, "error", err)
		return
	}
	if ifaceDelta { // reduce number of unneeded updates, by only sending on iface changes
		if !newNode.Connected {
			publishDeletedNodeUpdate = true
		}
		publishPeerUpdate = true
		// reset failover data for this node
		ResetFailedOverPeer(newNode)
		ResetAutoRelayedPeer(newNode)
	}
	return
}

// RemoveHost - removes a given host from server
func RemoveHost(h *schema.Host, forceDelete bool) error {
	if !forceDelete && len(h.Nodes) > 0 {
		return fmt.Errorf("host still has associated nodes")
	}

	if len(h.Nodes) > 0 {
		if err := DisassociateAllNodesFromHost(h.ID.String()); err != nil {
			return err
		}
	}

	err := h.Delete(db.WithContext(context.TODO()))
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
func UpdateHostNetwork(h *schema.Host, network string, add bool) (*models.Node, error) {
	for _, nodeID := range h.Nodes {
		node, err := GetNodeByID(nodeID)
		if err != nil || node.PendingDelete {
			continue
		}
		if node.Network == network {
			if !add {
				return &node, nil
			} else {
				return &node, errors.New("host already part of network " + network)
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
func AssociateNodeToHost(n *models.Node, h *schema.Host) error {
	if len(h.ID.String()) == 0 || h.ID == uuid.Nil {
		return ErrInvalidHostID
	}
	n.HostID = h.ID
	err := createNode(n)
	if err != nil {
		return err
	}
	currentHost := &schema.Host{ID: h.ID}
	if err = currentHost.Get(db.WithContext(context.TODO())); err != nil {
		return err
	}
	h.HostPass = currentHost.HostPass
	h.Nodes = append(currentHost.Nodes, n.ID.String())
	return UpsertHost(h)
}

// DissasociateNodeFromHost - deletes a node and removes from host nodes
// should be the only way nodes are deleted as of 0.18
func DissasociateNodeFromHost(n *models.Node, h *schema.Host) error {
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
	host := &schema.Host{ID: uuid.MustParse(hostID)}
	if err := host.Get(db.WithContext(context.TODO())); err != nil {
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
func GetDefaultHosts() []schema.Host {
	defaultHostList := []schema.Host{}
	hosts, err := (&schema.Host{}).ListAll(db.WithContext(context.TODO()))
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
	currHost := &schema.Host{ID: uuid.MustParse(hostID)}
	if err := currHost.Get(db.WithContext(context.TODO())); err != nil {
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

// GetRelatedHosts - fetches related hosts of a given host
func GetRelatedHosts(hostID string) []schema.Host {
	relatedHosts := []schema.Host{}
	networks := GetHostNetworks(hostID)
	networkMap := make(map[string]struct{})
	for _, network := range networks {
		networkMap[network] = struct{}{}
	}
	hosts, err := (&schema.Host{}).ListAll(db.WithContext(context.TODO()))
	if err == nil {
		for _, host := range hosts {
			if host.ID.String() == hostID {
				continue
			}
			networks := GetHostNetworks(host.ID.String())
			for _, network := range networks {
				if _, ok := networkMap[network]; ok {
					relatedHosts = append(relatedHosts, host)
					break
				}
			}
		}
	}
	return relatedHosts
}

// CheckHostPort checks host endpoints to ensures that hosts on the same server
// with the same endpoint have different listen ports
// in the case of 64535 hosts or more with same endpoint, ports will not be changed
func CheckHostPorts(h *schema.Host) (changed bool) {
	if h.IsStaticPort {
		return false
	}
	if h.EndpointIP == nil {
		return
	}

	// Get the current host from database to check if it already has a valid port assigned
	// This check happens before the mutex to avoid unnecessary locking
	currentHost := &schema.Host{ID: h.ID}
	err := currentHost.Get(db.WithContext(context.TODO()))
	if err == nil && currentHost.ListenPort > 0 {
		// If the host already has a port in the database, use that instead of the incoming port
		// This prevents the host from being reassigned when the client sends the old port
		if currentHost.ListenPort != h.ListenPort {
			h.ListenPort = currentHost.ListenPort
		}
	}

	// Only acquire mutex when we need to check for port conflicts
	// This reduces contention for the common case where ports are already valid
	hostPortMutex.Lock()
	defer hostPortMutex.Unlock()

	originalPort := h.ListenPort
	defer func() {
		if originalPort != h.ListenPort {
			changed = true
		}
	}()

	hosts, err := (&schema.Host{}).ListAll(db.WithContext(context.TODO()))
	if err != nil {
		return
	}

	// Build map of ports in use by other hosts with the same endpoint
	portsInUse := make(map[int]bool)
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
		if host.ListenPort > 0 {
			portsInUse[host.ListenPort] = true
		}
	}

	// If current port is not in use, no change needed
	if !portsInUse[h.ListenPort] {
		return
	}

	// Find an available port
	maxIterations := maxPort - minPort + 1
	checkedPorts := make(map[int]bool)
	initialPort := h.ListenPort

	for i := 0; i < maxIterations; i++ {
		// Special case: skip port 443 by jumping to 51821
		if h.ListenPort == 443 {
			h.ListenPort = 51821
		} else {
			h.ListenPort++
		}

		// Wrap around if we exceed maxPort
		if h.ListenPort > maxPort {
			h.ListenPort = minPort
		}

		// Avoid infinite loop - if we've checked this port before, we've cycled through all
		if checkedPorts[h.ListenPort] {
			// All ports are in use, keep original port
			h.ListenPort = originalPort
			break
		}
		checkedPorts[h.ListenPort] = true

		// Re-read hosts to get the latest state (in case another host just changed its port)
		// This is important to avoid conflicts when multiple hosts are being processed
		latestHosts, err := (&schema.Host{}).ListAll(db.WithContext(context.TODO()))
		if err == nil {
			// Update portsInUse with latest state
			for _, host := range latestHosts {
				if host.ID.String() == h.ID.String() {
					continue
				}
				if host.EndpointIP == nil {
					continue
				}
				if !host.EndpointIP.Equal(h.EndpointIP) {
					continue
				}
				if host.ListenPort > 0 {
					portsInUse[host.ListenPort] = true
				}
			}
		}

		// If this port is not in use, we found an available port
		if !portsInUse[h.ListenPort] {
			break
		}

		// If we've wrapped back to the initial port, all ports are in use
		if h.ListenPort == initialPort && i > 0 {
			h.ListenPort = originalPort
			break
		}
	}

	return
}

// HostExists - checks if given host already exists
func HostExists(h *schema.Host) bool {
	_host := &schema.Host{ID: h.ID}
	err := _host.Get(db.WithContext(context.TODO()))
	return (err != nil && !errors.Is(err, gorm.ErrRecordNotFound)) || (err == nil)
}

// GetHostByNodeID - returns a host if found to have a node's ID, else nil
func GetHostByNodeID(id string) *schema.Host {
	hosts, err := (&schema.Host{}).ListAll(db.WithContext(context.TODO()))
	if err != nil {
		return nil
	}
	for i := range hosts {
		h := hosts[i]
		if StringSliceContains(h.Nodes, id) {
			return &h
		}
	}
	return nil
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
