package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/goombaio/namegenerator"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic/acls"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
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
		if extclient.Mutex == nil {
			extclient.Mutex = &sync.Mutex{}
		}
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
	if extclient.Mutex == nil {
		extclient.Mutex = &sync.Mutex{}
	}
	extClientCacheMutex.RUnlock()
	return
}

func storeExtClientInCache(key string, extclient models.ExtClient) {
	extClientCacheMutex.Lock()
	if extclient.Mutex == nil {
		extclient.Mutex = &sync.Mutex{}
	}
	extClientCacheMap[key] = extclient
	extClientCacheMutex.Unlock()
}

// ExtClient.GetEgressRangesOnNetwork - returns the egress ranges on network of ext client
func GetEgressRangesOnNetwork(client *models.ExtClient) ([]string, error) {

	var result []string
	eli, _ := (&schema.Egress{Network: client.Network}).ListByNetwork(db.WithContext(context.TODO()))
	staticNode := client.ConvertToStaticNode()
	userPolicies := ListUserPolicies(models.NetworkID(client.Network))
	defaultUserPolicy, _ := GetDefaultPolicy(models.NetworkID(client.Network), models.UserPolicy)

	for _, eI := range eli {
		if !eI.Status {
			continue
		}
		if eI.Domain == "" && eI.Range == "" {
			continue
		}
		if eI.Domain != "" && len(eI.DomainAns) == 0 {
			continue
		}
		rangesToBeAdded := []string{}
		if eI.Domain != "" {
			rangesToBeAdded = append(rangesToBeAdded, eI.DomainAns...)
		} else {
			rangesToBeAdded = append(rangesToBeAdded, eI.Range)
		}
		if defaultUserPolicy.Enabled {
			result = append(result, rangesToBeAdded...)
		} else {
			if staticNode.IsUserNode && staticNode.StaticNode.OwnerID != "" {
				user, err := GetUser(staticNode.StaticNode.OwnerID)
				if err != nil {
					return []string{}, errors.New("user not found")
				}
				if DoesUserHaveAccessToEgress(user, &eI, userPolicies) {
					result = append(result, rangesToBeAdded...)
				}
			} else {
				result = append(result, rangesToBeAdded...)
			}
		}

	}
	extclients, _ := GetNetworkExtClients(client.Network)
	for _, extclient := range extclients {
		if extclient.ClientID == client.ClientID {
			continue
		}
		result = append(result, extclient.ExtraAllowedIPs...)
	}

	return UniqueIPNetStrList(result), nil
}

// UniqueIPNetList deduplicates and sorts a list of CIDR strings.
func UniqueIPNetStrList(ipnets []string) []string {
	uniqueMap := make(map[string]struct{})

	for _, cidr := range ipnets {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue // skip invalid CIDR strings
		}
		key := ipnet.String() // normalized CIDR
		uniqueMap[key] = struct{}{}
	}

	// Convert map keys to slice
	uniqueList := make([]string, 0, len(uniqueMap))
	for cidr := range uniqueMap {
		uniqueList = append(uniqueList, cidr)
	}

	sort.Strings(uniqueList)
	return uniqueList
}

// DeleteExtClient - deletes an existing ext client
func DeleteExtClient(network string, clientid string, isUpdate bool) error {
	key, err := GetRecordKey(clientid, network)
	if err != nil {
		return err
	}
	extClient, err := GetExtClient(clientid, network)
	if err != nil {
		return err
	}
	err = database.DeleteRecord(database.EXT_CLIENT_TABLE_NAME, key)
	if err != nil {
		return err
	}
	if servercfg.CacheEnabled() {
		// recycle ip address
		if extClient.Address != "" {
			RemoveIpFromAllocatedIpMap(network, extClient.Address)
		}
		if extClient.Address6 != "" {
			RemoveIpFromAllocatedIpMap(network, extClient.Address6)
		}
		deleteExtClientFromCache(key)
	}
	if !isUpdate && extClient.RemoteAccessClientID != "" {
		LogEvent(&models.Event{
			Action: models.Disconnect,
			Source: models.Subject{
				ID:   extClient.OwnerID,
				Name: extClient.OwnerID,
				Type: models.UserSub,
			},
			TriggeredBy: extClient.OwnerID,
			Target: models.Subject{
				ID:   extClient.Network,
				Name: extClient.Network,
				Type: models.NetworkSub,
				Info: extClient,
			},
			NetworkID: models.NetworkID(extClient.Network),
			Origin:    models.ClientApp,
		})
	}
	go RemoveNodeFromAclPolicy(extClient.ConvertToStaticNode())
	return nil
}

// DeleteExtClientAndCleanup - deletes an existing ext client and update ACLs
func DeleteExtClientAndCleanup(extClient models.ExtClient) error {

	//delete extClient record
	err := DeleteExtClient(extClient.Network, extClient.ClientID, false)
	if err != nil {
		slog.Error("DeleteExtClientAndCleanup-remove extClient record: ", "Error", err.Error())
		return err
	}

	//update ACLs
	var networkAcls acls.ACLContainer
	networkAcls, err = networkAcls.Get(acls.ContainerID(extClient.Network))
	if err != nil {
		slog.Error("DeleteExtClientAndCleanup-update network acls: ", "Error", err.Error())
		return err
	}
	for objId := range networkAcls {
		delete(networkAcls[objId], acls.AclID(extClient.ClientID))
	}
	delete(networkAcls, acls.AclID(extClient.ClientID))
	if _, err = networkAcls.Save(acls.ContainerID(extClient.Network)); err != nil {
		slog.Error("DeleteExtClientAndCleanup-update network acls:", "Error", err.Error())
		return err
	}

	return nil
}

//TODO - enforce extclient-to-extclient on ingress gw
/* 1. fetch all non-user static nodes
a. check against each user node, if allowed add rule

*/

// GetNetworkExtClients - gets the ext clients of given network
func GetNetworkExtClients(network string) ([]models.ExtClient, error) {
	var extclients []models.ExtClient
	if servercfg.CacheEnabled() {
		allextclients := getAllExtClientsFromCache()
		if len(allextclients) != 0 {
			for _, extclient := range allextclients {
				if extclient.Network == network {
					extclients = append(extclients, extclient)
				}
			}
			return extclients, nil
		}
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
		key, err := GetRecordKey(extclient.ClientID, extclient.Network)
		if err == nil {
			if servercfg.CacheEnabled() {
				storeExtClientInCache(key, extclient)
			}
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
	if servercfg.CacheEnabled() {
		if extclient, ok := getExtClientFromCache(key); ok {
			return extclient, nil
		}
	}
	data, err := database.FetchRecord(database.EXT_CLIENT_TABLE_NAME, key)
	if err != nil {
		return extclient, err
	}
	err = json.Unmarshal([]byte(data), &extclient)
	if servercfg.CacheEnabled() {
		storeExtClientInCache(key, extclient)
	}
	return extclient, err
}

// GetGwExtclients - return all ext clients attached to the passed gw id
func GetGwExtclients(nodeID, network string) []models.ExtClient {
	gwClients := []models.ExtClient{}
	clients, err := GetNetworkExtClients(network)
	if err != nil {
		return gwClients
	}
	for _, client := range clients {
		if client.IngressGatewayID == nodeID {
			gwClients = append(gwClients, client)
		}
	}
	return gwClients
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
	if extclient.ExtraAllowedIPs == nil {
		extclient.ExtraAllowedIPs = []string{}
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
		extclient.ClientID, err = GenerateNodeName(extclient.Network)
		if err != nil {
			return err
		}
	}

	extclient.LastModified = time.Now().Unix()
	return SaveExtClient(extclient)
}

// GenerateNodeName - generates a random node name
func GenerateNodeName(network string) (string, error) {
	seed := time.Now().UTC().UnixNano()
	nameGenerator := namegenerator.NewNameGenerator(seed)
	var name string
	cnt := 0
	for {
		if cnt > 10 {
			return "", errors.New("couldn't generate random name, try again")
		}
		cnt += 1
		name = nameGenerator.Generate()
		if len(name) > 15 {
			continue
		}
		_, err := GetExtClient(name, network)
		if err == nil {
			// config exists with same name
			continue
		}
		break
	}
	return name, nil
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
	if servercfg.CacheEnabled() {
		storeExtClientInCache(key, *extclient)
		if _, ok := allocatedIpMap[extclient.Network]; ok {
			if extclient.Address != "" {
				AddIpToAllocatedIpMap(extclient.Network, net.ParseIP(extclient.Address))
			}
			if extclient.Address6 != "" {
				AddIpToAllocatedIpMap(extclient.Network, net.ParseIP(extclient.Address6))
			}
		}
	}

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
	new.ExtraAllowedIPs = update.ExtraAllowedIPs
	if update.DeniedACLs != nil && !reflect.DeepEqual(old.DeniedACLs, update.DeniedACLs) {
		new.DeniedACLs = update.DeniedACLs
	}
	// replace any \r\n with \n in postup and postdown from HTTP request
	new.PostUp = strings.Replace(update.PostUp, "\r\n", "\n", -1)
	new.PostDown = strings.Replace(update.PostDown, "\r\n", "\n", -1)
	new.Tags = update.Tags
	if update.Location != "" && update.Location != old.Location {
		new.Location = update.Location
	}
	if update.Country != "" && update.Country != old.Country {
		new.Country = update.Country
	}
	if update.DeviceID != "" && old.DeviceID == "" {
		new.DeviceID = update.DeviceID
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

// GetAllExtClientsWithStatus - returns all external clients with
// given status.
func GetAllExtClientsWithStatus(status models.NodeStatus) ([]models.ExtClient, error) {
	extClients, err := GetAllExtClients()
	if err != nil {
		return nil, err
	}

	var validExtClients []models.ExtClient
	for _, extClient := range extClients {
		node := extClient.ConvertToStaticNode()
		GetNodeStatus(&node, false)

		if node.Status == status {
			validExtClients = append(validExtClients, extClient)
		}
	}

	return validExtClients, nil
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
	if err := DeleteExtClient(client.Network, client.ClientID, true); err != nil {
		slog.Error("failed to delete ext client during update", "id", client.ClientID, "network", client.Network, "error", err)
		return newClient, err
	}
	if err := SaveExtClient(&newClient); err != nil {
		slog.Error("failed to save updated ext client during update", "id", newClient.ClientID, "network", newClient.Network, "error", err)
		return newClient, err
	}

	return newClient, nil
}

func GetExtPeers(node, peer *models.Node) ([]wgtypes.PeerConfig, []models.IDandAddr, []models.EgressNetworkRoutes, error) {
	var peers []wgtypes.PeerConfig
	var idsAndAddr []models.IDandAddr
	var egressRoutes []models.EgressNetworkRoutes
	extPeers, err := GetNetworkExtClients(node.Network)
	if err != nil {
		return peers, idsAndAddr, egressRoutes, err
	}
	host, err := GetHost(node.HostID.String())
	if err != nil {
		return peers, idsAndAddr, egressRoutes, err
	}
	for _, extPeer := range extPeers {
		extPeer := extPeer
		if !IsClientNodeAllowed(&extPeer, peer.ID.String()) {
			continue
		}
		if extPeer.RemoteAccessClientID == "" {
			if ok := IsPeerAllowed(extPeer.ConvertToStaticNode(), *peer, true); !ok {
				continue
			}
		} else {
			if ok, _ := IsUserAllowedToCommunicate(extPeer.OwnerID, *peer); !ok {
				continue
			}
		}

		pubkey, err := wgtypes.ParseKey(extPeer.PublicKey)
		if err != nil {
			logger.Log(1, "error parsing ext pub key:", err.Error())
			continue
		}

		if host.PublicKey.String() == extPeer.PublicKey ||
			extPeer.IngressGatewayID != node.ID.String() || !extPeer.Enabled {
			continue
		}

		var allowedips []net.IPNet
		var peer wgtypes.PeerConfig
		if extPeer.Address != "" {
			var peeraddr = net.IPNet{
				IP:   net.ParseIP(extPeer.Address),
				Mask: net.CIDRMask(32, 32),
			}
			if peeraddr.IP != nil && peeraddr.Mask != nil {
				allowedips = append(allowedips, peeraddr)
			}
		}

		if extPeer.Address6 != "" {
			var addr6 = net.IPNet{
				IP:   net.ParseIP(extPeer.Address6),
				Mask: net.CIDRMask(128, 128),
			}
			if addr6.IP != nil && addr6.Mask != nil {
				allowedips = append(allowedips, addr6)
			}
		}
		for _, extraAllowedIP := range extPeer.ExtraAllowedIPs {
			_, cidr, err := net.ParseCIDR(extraAllowedIP)
			if err == nil {
				allowedips = append(allowedips, *cidr)
			}
		}
		egressRoutes = append(egressRoutes, getExtPeerEgressRoute(*node, extPeer)...)
		primaryAddr := extPeer.Address
		if primaryAddr == "" {
			primaryAddr = extPeer.Address6
		}
		peer = wgtypes.PeerConfig{
			PublicKey:         pubkey,
			ReplaceAllowedIPs: true,
			AllowedIPs:        allowedips,
		}
		peers = append(peers, peer)
		idsAndAddr = append(idsAndAddr, models.IDandAddr{
			ID:          peer.PublicKey.String(),
			Name:        extPeer.ClientID,
			Address:     primaryAddr,
			IsExtClient: true,
		})
	}
	return peers, idsAndAddr, egressRoutes, nil

}

func getExtPeerEgressRoute(node models.Node, extPeer models.ExtClient) (egressRoutes []models.EgressNetworkRoutes) {
	r := models.EgressNetworkRoutes{
		PeerKey:       extPeer.PublicKey,
		EgressGwAddr:  extPeer.AddressIPNet4(),
		EgressGwAddr6: extPeer.AddressIPNet6(),
		NodeAddr:      node.Address,
		NodeAddr6:     node.Address6,
		EgressRanges:  extPeer.ExtraAllowedIPs,
	}
	for _, extraAllowedIP := range extPeer.ExtraAllowedIPs {
		r.EgressRangesWithMetric = append(r.EgressRangesWithMetric, models.EgressRangeMetric{
			Network:     extraAllowedIP,
			RouteMetric: 256,
		})
	}
	egressRoutes = append(egressRoutes, r)
	return
}

func getExtpeerEgressRanges(node models.Node) (ranges, ranges6 []net.IPNet) {
	extPeers, err := GetNetworkExtClients(node.Network)
	if err != nil {
		return
	}
	for _, extPeer := range extPeers {
		if len(extPeer.ExtraAllowedIPs) == 0 {
			continue
		}
		if ok, _ := IsNodeAllowedToCommunicate(extPeer.ConvertToStaticNode(), node, true); !ok {
			continue
		}
		for _, allowedRange := range extPeer.ExtraAllowedIPs {
			_, ipnet, err := net.ParseCIDR(allowedRange)
			if err == nil {
				if ipnet.IP.To4() != nil {
					ranges = append(ranges, *ipnet)
				} else {
					ranges6 = append(ranges6, *ipnet)
				}

			}
		}
	}
	return
}

func getExtpeersExtraRoutes(node models.Node) (egressRoutes []models.EgressNetworkRoutes) {
	extPeers, err := GetNetworkExtClients(node.Network)
	if err != nil {
		return
	}
	for _, extPeer := range extPeers {
		if len(extPeer.ExtraAllowedIPs) == 0 {
			continue
		}
		if ok, _ := IsNodeAllowedToCommunicate(extPeer.ConvertToStaticNode(), node, true); !ok {
			continue
		}
		egressRoutes = append(egressRoutes, getExtPeerEgressRoute(node, extPeer)...)
	}
	return
}

func GetExtclientAllowedIPs(client models.ExtClient) (allowedIPs []string) {
	gwnode, err := GetNodeByID(client.IngressGatewayID)
	if err != nil {
		logger.Log(0,
			fmt.Sprintf("failed to get ingress gateway node [%s] info: %v", client.IngressGatewayID, err))
		return
	}

	network, err := GetParentNetwork(client.Network)
	if err != nil {
		logger.Log(1, "Could not retrieve Ingress Gateway Network", client.Network)
		return
	}
	if IsInternetGw(gwnode) {
		egressrange := "0.0.0.0/0"
		if gwnode.Address6.IP != nil && client.Address6 != "" {
			egressrange += "," + "::/0"
		}
		allowedIPs = []string{egressrange}
	} else {
		allowedIPs = []string{network.AddressRange}

		if network.AddressRange6 != "" {
			allowedIPs = append(allowedIPs, network.AddressRange6)
		}
		if egressGatewayRanges, err := GetEgressRangesOnNetwork(&client); err == nil {
			allowedIPs = append(allowedIPs, egressGatewayRanges...)
		}
	}
	return
}

func GetStaticNodesByNetwork(network models.NetworkID, onlyWg bool) (staticNode []models.Node) {
	extClients, err := GetAllExtClients()
	if err != nil {
		return
	}
	SortExtClient(extClients[:])
	for _, extI := range extClients {
		if extI.Network == network.String() {
			if onlyWg && extI.RemoteAccessClientID != "" {
				continue
			}
			staticNode = append(staticNode, extI.ConvertToStaticNode())
		}
	}

	return
}
