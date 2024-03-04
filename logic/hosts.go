package logic

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

var (
	hostCacheMutex = &sync.RWMutex{}
	hostsCacheMap  = make(map[string]models.Host)
)

var (
	// ErrHostExists error indicating that host exists when trying to create new host
	ErrHostExists error = errors.New("host already exists")
	// ErrInvalidHostID
	ErrInvalidHostID error = errors.New("invalid host id")
)

func getHostsFromCache() (hosts []models.Host) {
	hostCacheMutex.RLock()
	for _, host := range hostsCacheMap {
		hosts = append(hosts, host)
	}
	hostCacheMutex.RUnlock()
	return
}

func getHostsMapFromCache() (hostsMap map[string]models.Host) {
	hostCacheMutex.RLock()
	hostsMap = hostsCacheMap
	hostCacheMutex.RUnlock()
	return
}

func getHostFromCache(hostID string) (host models.Host, ok bool) {
	hostCacheMutex.RLock()
	host, ok = hostsCacheMap[hostID]
	hostCacheMutex.RUnlock()
	return
}

func storeHostInCache(h models.Host) {
	hostCacheMutex.Lock()
	hostsCacheMap[h.ID.String()] = h
	hostCacheMutex.Unlock()
}

func deleteHostFromCache(hostID string) {
	hostCacheMutex.Lock()
	delete(hostsCacheMap, hostID)
	hostCacheMutex.Unlock()
}

func loadHostsIntoCache(hMap map[string]models.Host) {
	hostCacheMutex.Lock()
	hostsCacheMap = hMap
	hostCacheMutex.Unlock()
}

const (
	maxPort = 1<<16 - 1
	minPort = 1025
)

// GetAllHosts - returns all hosts in flat list or error
func GetAllHosts() ([]models.Host, error) {
	var currHosts []models.Host
	if servercfg.CacheEnabled() {
		currHosts := getHostsFromCache()
		if len(currHosts) != 0 {
			return currHosts, nil
		}
	}
	records, err := database.FetchRecords(database.HOSTS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return nil, err
	}
	currHostsMap := make(map[string]models.Host)
	if servercfg.CacheEnabled() {
		defer loadHostsIntoCache(currHostsMap)
	}
	for k := range records {
		var h models.Host
		err = json.Unmarshal([]byte(records[k]), &h)
		if err != nil {
			return nil, err
		}
		currHosts = append(currHosts, h)
		currHostsMap[h.ID.String()] = h
	}

	return currHosts, nil
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

// GetHostsMap - gets all the current hosts on machine in a map
func GetHostsMap() (map[string]models.Host, error) {
	if servercfg.CacheEnabled() {
		hostsMap := getHostsMapFromCache()
		if len(hostsMap) != 0 {
			return hostsMap, nil
		}
	}
	records, err := database.FetchRecords(database.HOSTS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return nil, err
	}
	currHostMap := make(map[string]models.Host)
	if servercfg.CacheEnabled() {
		defer loadHostsIntoCache(currHostMap)
	}
	for k := range records {
		var h models.Host
		err = json.Unmarshal([]byte(records[k]), &h)
		if err != nil {
			return nil, err
		}
		currHostMap[h.ID.String()] = h
	}

	return currHostMap, nil
}

// GetHost - gets a host from db given id
func GetHost(hostid string) (*models.Host, error) {
	if servercfg.CacheEnabled() {
		if host, ok := getHostFromCache(hostid); ok {
			return &host, nil
		}
	}
	record, err := database.FetchRecord(database.HOSTS_TABLE_NAME, hostid)
	if err != nil {
		return nil, err
	}

	var h models.Host
	if err = json.Unmarshal([]byte(record), &h); err != nil {
		return nil, err
	}
	if servercfg.CacheEnabled() {
		storeHostInCache(h)
	}

	return &h, nil
}

// GetHostByPubKey - gets a host from db given pubkey
func GetHostByPubKey(hostPubKey string) (*models.Host, error) {
	hosts, err := GetAllHosts()
	if err != nil {
		return nil, err
	}
	for _, host := range hosts {
		if host.PublicKey.String() == hostPubKey {
			return &host, nil
		}
	}
	return nil, errors.New("host not found")
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
	if (err != nil && !database.IsEmptyRecord(err)) || (err == nil) {
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
	if currHost.EndpointIP.String() != newHost.EndpointIP.String() {
		currHost.EndpointIP = newHost.EndpointIP
		sendPeerUpdate = true
	}
	currHost.DaemonInstalled = newHost.DaemonInstalled
	currHost.Debug = newHost.Debug
	currHost.Verbosity = newHost.Verbosity
	currHost.Version = newHost.Version
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
	data, err := json.Marshal(h)
	if err != nil {
		return err
	}
	err = database.Insert(h.ID.String(), string(data), database.HOSTS_TABLE_NAME)
	if err != nil {
		return err
	}
	if servercfg.CacheEnabled() {
		storeHostInCache(*h)
	}

	return nil
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

	err := database.DeleteRecord(database.HOSTS_TABLE_NAME, h.ID.String())
	if err != nil {
		return err
	}
	if servercfg.CacheEnabled() {
		deleteHostFromCache(h.ID.String())
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

	err := database.DeleteRecord(database.HOSTS_TABLE_NAME, hostID)
	if err != nil {
		return err
	}
	if servercfg.CacheEnabled() {
		deleteHostFromCache(hostID)
	}
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
	logger.Log(0, "Hereeeee-----> 4.7.1")
	if n.HostID != h.ID { // check if node actually belongs to host
		return fmt.Errorf("node is not associated with host")
	}
	if len(h.Nodes) == 0 {
		return fmt.Errorf("no nodes present in given host")
	}
	index := -1
	for i := range h.Nodes {
		if h.Nodes[i] == n.ID.String() {
			index = i
			break
		}
	}
	logger.Log(0, "Hereeeee-----> 4.7.2")
	if index < 0 {
		if len(h.Nodes) == 0 {
			return fmt.Errorf("node %s, not found in host, %s", n.ID.String(), h.ID.String())
		}
	} else {
		h.Nodes = RemoveStringSlice(h.Nodes, index)
	}
	logger.Log(0, "Hereeeee-----> 4.7.3")
	go func() {
		if servercfg.IsPro {
			//logger.Log(0, "Hereeeee-----> 4.7.3.1")
			if clients, err := GetNetworkExtClients(n.Network); err != nil {
				//logger.Log(0, "Hereeeee-----> 4.7.3.2")
				for i := range clients {
					AllowClientNodeAccess(&clients[i], n.ID.String())
					//logger.Log(0, "Hereeeee-----> 4.7.3.3")
				}
				//logger.Log(0, "Hereeeee-----> 4.7.3.4")
			}
			//logger.Log(0, "Hereeeee-----> 4.7.3.5")

		}
	}()
	logger.Log(0, "Hereeeee-----> 4.7.4")
	if err := DeleteNodeByID(n); err != nil {
		return err
	}
	logger.Log(0, "Hereeeee-----> 4.7.5")
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
			logger.Log(0, "failed to get host node", err.Error())
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

// GetRelatedHosts - fetches related hosts of a given host
func GetRelatedHosts(hostID string) []models.Host {
	relatedHosts := []models.Host{}
	networks := GetHostNetworks(hostID)
	networkMap := make(map[string]struct{})
	for _, network := range networks {
		networkMap[network] = struct{}{}
	}
	hosts, err := GetAllHosts()
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
	return (err != nil && !database.IsEmptyRecord(err)) || (err == nil)
}

// GetHostByNodeID - returns a host if found to have a node's ID, else nil
func GetHostByNodeID(id string) *models.Host {
	hosts, err := GetAllHosts()
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
