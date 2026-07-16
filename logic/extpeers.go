package logic

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/goombaio/namegenerator"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slog"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// extClientCacheMap maps tenant ID -> *sync.Map of record key -> models.ExtClient
var extClientCacheMap sync.Map

// getTenantExtClientCache returns the ext client cache map for the given tenant, creating it if necessary
func getTenantExtClientCache(tenantID string) *sync.Map {
	v, _ := extClientCacheMap.LoadOrStore(tenantID, &sync.Map{})
	return v.(*sync.Map)
}

func getAllExtClientsFromCache(ctx context.Context) (extClients []models.ExtClient) {
	getTenantExtClientCache(scope.ID(ctx)).Range(func(_, v any) bool {
		extclient := v.(models.ExtClient)
		if extclient.Mutex == nil {
			extclient.Mutex = &sync.Mutex{}
		}
		extClients = append(extClients, extclient)
		return true
	})
	return
}

func deleteExtClientFromCache(ctx context.Context, key string) {
	getTenantExtClientCache(scope.ID(ctx)).Delete(key)
}

func getExtClientFromCache(ctx context.Context, key string) (extclient models.ExtClient, ok bool) {
	v, ok := getTenantExtClientCache(scope.ID(ctx)).Load(key)
	if !ok {
		return extclient, false
	}
	extclient = v.(models.ExtClient)
	if extclient.Mutex == nil {
		extclient.Mutex = &sync.Mutex{}
	}
	return extclient, true
}

func storeExtClientInCache(ctx context.Context, key string, extclient models.ExtClient) {
	if extclient.Mutex == nil {
		extclient.Mutex = &sync.Mutex{}
	}
	getTenantExtClientCache(scope.ID(ctx)).Store(key, extclient)
}

// ExtClient.GetEgressRangesOnNetwork - returns the egress ranges on network of ext client
func GetEgressRangesOnNetwork(ctx context.Context, client *models.ExtClient) ([]string, error) {

	var result []string
	eli, _ := (&schema.Egress{Network: client.Network}).ListByNetwork(ctx)
	staticNode := models.ConvertToStaticNode(*client)
	userPolicies := ListUserPolicies(ctx, schema.NetworkID(client.Network))
	defaultUserPolicy, _ := GetDefaultPolicy(ctx, schema.NetworkID(client.Network), models.UserPolicy)

	for _, eI := range eli {
		if !eI.Status {
			continue
		}
		if !IsDomainBasedEgress(eI) && eI.Range == "" {
			continue
		}
		if IsDomainBasedEgress(eI) && !HasEgressDomainAns(eI) {
			continue
		}
		rangesToBeAdded := []string{}
		if IsDomainBasedEgress(eI) {
			rangesToBeAdded = append(rangesToBeAdded, AllDomainAnsFromEgress(eI)...)
		} else {
			// Use virtual NAT range if enabled, otherwise use original range
			egressRange := eI.Range
			if eI.Nat && eI.VirtualRange != "" {
				egressRange = eI.VirtualRange
			}
			rangesToBeAdded = append(rangesToBeAdded, egressRange)
		}
		if defaultUserPolicy.Enabled {
			result = append(result, rangesToBeAdded...)
		} else {
			if staticNode.IsUserNode && staticNode.StaticNode.OwnerID != "" {
				user := &schema.User{Username: staticNode.StaticNode.OwnerID}
				err := user.Get(ctx)
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
	extclients, _ := GetNetworkExtClients(ctx, client.Network)
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
func DeleteExtClient(ctx context.Context, network string, clientid string, isUpdate bool) error {
	key, err := GetRecordKey(clientid, network)
	if err != nil {
		return err
	}
	extClient, err := GetExtClient(ctx, clientid, network)
	if err != nil {
		return err
	}
	if err = (&schema.ExtClientRecord{Key: key}).Delete(ctx); err != nil {
		return err
	}
	if servercfg.CacheEnabled() {
		deleteExtClientFromCache(ctx, key)
	}
	if !isUpdate && extClient.RemoteAccessClientID != "" {
		LogEvent(ctx, &models.Event{
			Action: schema.Disconnect,
			Source: models.Subject{
				ID:   extClient.OwnerID,
				Name: extClient.OwnerID,
				Type: schema.UserSub,
			},
			TriggeredBy: extClient.OwnerID,
			Target: models.Subject{
				ID:   extClient.Network,
				Name: extClient.Network,
				Type: schema.NetworkSub,
				Info: extClient,
			},
			NetworkID: schema.NetworkID(extClient.Network),
			Origin:    schema.ClientApp,
		})
	}
	detachedCtx := scope.WithContext(db.WithContext(context.Background()), scope.Level(ctx), scope.ID(ctx))
	go RemoveNodeFromAclPolicy(detachedCtx, models.ConvertToStaticNode(extClient))
	return nil
}

// DeleteExtClientAndCleanup - deletes an existing ext client and update ACLs
func DeleteExtClientAndCleanup(ctx context.Context, extClient models.ExtClient) error {

	//delete extClient record
	err := DeleteExtClient(ctx, extClient.Network, extClient.ClientID, false)
	if err != nil {
		slog.Error("DeleteExtClientAndCleanup-remove extClient record: ", "Error", err.Error())
		return err
	}

	return nil
}

//TODO - enforce extclient-to-extclient on ingress gw
/* 1. fetch all non-user static nodes
a. check against each user node, if allowed add rule

*/

// GetNetworkExtClients - gets the ext clients of given network
func GetNetworkExtClients(ctx context.Context, network string) ([]models.ExtClient, error) {
	var extclients []models.ExtClient
	if servercfg.CacheEnabled() {
		allextclients := getAllExtClientsFromCache(ctx)
		if len(allextclients) != 0 {
			for _, extclient := range allextclients {
				if extclient.Network == network {
					extclients = append(extclients, extclient)
				}
			}
			return extclients, nil
		}
	}
	records, err := (&schema.ExtClientRecord{}).List(ctx)
	if err != nil {
		return extclients, err
	}
	for _, r := range records {
		extclient := r.Value.Data()
		key, err := GetRecordKey(extclient.ClientID, extclient.Network)
		if err == nil && servercfg.CacheEnabled() {
			storeExtClientInCache(ctx, key, extclient)
		}
		if extclient.Network == network {
			extclients = append(extclients, extclient)
		}
	}
	return extclients, nil
}

// GetExtClient - gets a single ext client on a network
func GetExtClient(ctx context.Context, clientid string, network string) (models.ExtClient, error) {
	var extclient models.ExtClient
	key, err := GetRecordKey(clientid, network)
	if err != nil {
		return extclient, err
	}
	if servercfg.CacheEnabled() {
		if extclient, ok := getExtClientFromCache(ctx, key); ok {
			return extclient, nil
		}
	}
	r := &schema.ExtClientRecord{Key: key}
	if err = r.Get(ctx); err != nil {
		return extclient, err
	}
	extclient = r.Value.Data()
	if servercfg.CacheEnabled() {
		storeExtClientInCache(ctx, key, extclient)
	}
	return extclient, nil
}

func GenerateNodeName(ctx context.Context, network string) (string, error) {
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
		_, err := GetExtClient(ctx, name, network)
		if err == nil {
			// config exists with same name
			continue
		}
		break
	}
	return name, nil
}

// SaveExtClient - saves an ext client to database
func SaveExtClient(ctx context.Context, extclient *models.ExtClient) error {
	key, err := GetRecordKey(extclient.ClientID, extclient.Network)
	if err != nil {
		return err
	}
	r := &schema.ExtClientRecord{Key: key, Value: datatypes.NewJSONType(*extclient)}
	if err = r.Upsert(ctx); err != nil {
		return err
	}
	if servercfg.CacheEnabled() {
		storeExtClientInCache(ctx, key, *extclient)
	}
	return SetNetworkNodesLastModified(ctx, extclient.Network)
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
		new.Country = strings.ToUpper(update.Country)
	}
	if update.DeviceID != "" && old.DeviceID == "" {
		new.DeviceID = update.DeviceID
	}
	if update.OS != "" {
		new.OS = update.OS
	}
	if update.OSFamily != "" {
		new.OSFamily = update.OSFamily
	}
	if update.OSVersion != "" {
		new.OSVersion = update.OSVersion
	}
	if update.KernelVersion != "" {
		new.KernelVersion = update.KernelVersion
	}
	if update.ClientVersion != "" {
		new.ClientVersion = update.ClientVersion
	}
	return new
}

// GetExtClientsByID - gets the clients of attached gateway
func GetExtClientsByID(ctx context.Context, nodeid, network string) ([]models.ExtClient, error) {
	var result []models.ExtClient
	currentClients, err := GetNetworkExtClients(ctx, network)
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
func GetAllExtClients(ctx context.Context) ([]models.ExtClient, error) {
	var clients = []models.ExtClient{}
	currentNetworks, err := (&schema.Network{}).ListAll(ctx)
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return clients, nil
	} else if err != nil {
		return clients, err
	}

	for i := range currentNetworks {
		netName := currentNetworks[i].Name
		netClients, err := GetNetworkExtClients(ctx, netName)
		if err != nil {
			continue
		}
		clients = append(clients, netClients...)
	}

	return clients, nil
}

// GetAllExtClientsWithStatus - returns all external clients with
// given status.
func GetAllExtClientsWithStatus(ctx context.Context, status schema.NodeStatus) ([]models.ExtClient, error) {
	extClients, err := GetAllExtClients(ctx)
	if err != nil {
		return nil, err
	}

	var validExtClients []models.ExtClient
	for _, extClient := range extClients {
		if extClient.Status == status {
			validExtClients = append(validExtClients, extClient)
		}
	}

	return validExtClients, nil
}

// ToggleExtClientConnectivity - enables or disables an ext client
func ToggleExtClientConnectivity(ctx context.Context, client *models.ExtClient, enable bool) (models.ExtClient, error) {
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
	if err := DeleteExtClient(ctx, client.Network, client.ClientID, true); err != nil {
		slog.Error("failed to delete ext client during update", "id", client.ClientID, "network", client.Network, "error", err)
		return newClient, err
	}
	if err := SaveExtClient(ctx, &newClient); err != nil {
		slog.Error("failed to save updated ext client during update", "id", newClient.ClientID, "network", newClient.Network, "error", err)
		return newClient, err
	}

	return newClient, nil
}

func GetExtPeers(ctx context.Context, node, peer *models.Node, addressIdentityMap map[string]models.PeerIdentity) ([]wgtypes.PeerConfig, []models.IDandAddr, []models.EgressNetworkRoutes, error) {
	var skipFlowLogs bool
	if !GetFeatureFlags().EnableFlowLogs || !GetServerSettings(ctx).EnableFlowLogs {
		skipFlowLogs = true
	}
	var peers []wgtypes.PeerConfig
	var idsAndAddr []models.IDandAddr
	var egressRoutes []models.EgressNetworkRoutes
	extPeers, err := GetNetworkExtClients(ctx, node.Network)
	if err != nil {
		return peers, idsAndAddr, egressRoutes, err
	}
	host := &schema.Host{
		ID: node.HostID,
	}
	err = host.Get(ctx)
	if err != nil {
		return peers, idsAndAddr, egressRoutes, err
	}
	for _, extPeer := range extPeers {
		extPeer := extPeer
		if extPeer.RemoteAccessClientID == "" {
			if ok := IsPeerAllowed(ctx, models.ConvertToStaticNode(extPeer), *peer, true); !ok {
				continue
			}
		} else {
			if ok, _ := IsUserAllowedToCommunicate(ctx, extPeer.OwnerID, *peer); !ok {
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
		var extPeerAddr4, extPeerAddr6 net.IPNet
		if extPeer.Address != "" {
			extPeerAddr4 = net.IPNet{
				IP:   net.ParseIP(extPeer.Address),
				Mask: net.CIDRMask(32, 32),
			}
			if extPeerAddr4.IP != nil && extPeerAddr4.Mask != nil {
				allowedips = append(allowedips, extPeerAddr4)
			}
		}

		if extPeer.Address6 != "" {
			extPeerAddr6 = net.IPNet{
				IP:   net.ParseIP(extPeer.Address6),
				Mask: net.CIDRMask(128, 128),
			}
			if extPeerAddr6.IP != nil && extPeerAddr6.Mask != nil {
				allowedips = append(allowedips, extPeerAddr6)
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
		peerInfo := models.IDandAddr{
			ID:          peer.PublicKey.String(),
			Name:        extPeer.ClientID,
			Address:     primaryAddr,
			Address4:    extPeer.Address,
			Address6:    extPeer.Address6,
			IsExtClient: true,
		}
		if extPeer.DeviceID != "" || extPeer.RemoteAccessClientID != "" {
			peerInfo.UserName = extPeer.OwnerID
		}
		idsAndAddr = append(idsAndAddr, peerInfo)

		if !skipFlowLogs {
			if extPeerAddr4.IP != nil {
				peerID := extPeer.ClientID
				peerType := models.PeerType_WireGuard
				if extPeer.RemoteAccessClientID != "" {
					peerID = extPeer.OwnerID
					peerType = models.PeerType_User
				}

				addressIdentityMap[extPeerAddr4.IP.String()+"/32"] = models.PeerIdentity{
					ID:   peerID,
					Type: peerType,
					Name: peerID,
				}
			}

			if extPeerAddr6.IP != nil {
				peerID := extPeer.ClientID
				peerType := models.PeerType_WireGuard
				if extPeer.RemoteAccessClientID != "" {
					peerID = extPeer.OwnerID
					peerType = models.PeerType_User
				}

				addressIdentityMap[extPeerAddr6.IP.String()+"/128"] = models.PeerIdentity{
					ID:   peerID,
					Type: peerType,
					Name: peerID,
				}
			}
		}

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
		Network:       node.Network,
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

func getExtpeerEgressRanges(ctx context.Context, node models.Node) (ranges, ranges6 []net.IPNet) {
	extPeers, err := GetNetworkExtClients(ctx, node.Network)
	if err != nil {
		return
	}
	for _, extPeer := range extPeers {
		if len(extPeer.ExtraAllowedIPs) == 0 {
			continue
		}
		if ok, _ := IsNodeAllowedToCommunicate(ctx, models.ConvertToStaticNode(extPeer), node, true); !ok {
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

func getExtpeersExtraRoutes(ctx context.Context, node models.Node) (egressRoutes []models.EgressNetworkRoutes) {
	extPeers, err := GetNetworkExtClients(ctx, node.Network)
	if err != nil {
		return
	}
	for _, extPeer := range extPeers {
		if len(extPeer.ExtraAllowedIPs) == 0 || !extPeer.Enabled {
			continue
		}
		if ok, _ := IsNodeAllowedToCommunicate(ctx, models.ConvertToStaticNode(extPeer), node, true); !ok {
			continue
		}
		egressRoutes = append(egressRoutes, getExtPeerEgressRoute(node, extPeer)...)
	}
	return
}

func GetExtclientAllowedIPs(ctx context.Context, client models.ExtClient) (allowedIPs []string) {
	gwnode, err := GetNodeByID(client.IngressGatewayID)
	if err != nil {
		logger.Log(0,
			fmt.Sprintf("failed to get ingress gateway node [%s] info: %v", client.IngressGatewayID, err))
		return
	}

	network := &schema.Network{Name: client.Network}
	err = network.Get(ctx)
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
		if egressGatewayRanges, err := GetEgressRangesOnNetwork(ctx, &client); err == nil {
			allowedIPs = append(allowedIPs, egressGatewayRanges...)
		}
	}
	return
}

func GetStaticNodesByNetwork(ctx context.Context, network schema.NetworkID, onlyWg bool) (staticNode []models.Node) {
	extClients, err := GetAllExtClients(ctx)
	if err != nil {
		return
	}
	SortExtClient(extClients[:])
	for _, extI := range extClients {
		if extI.Network == network.String() {
			if onlyWg && extI.RemoteAccessClientID != "" {
				continue
			}
			staticNode = append(staticNode, models.ConvertToStaticNode(extI))
		}
	}

	return
}

// CleanupOtherExtclients cleans up other clients owned by the same use for the same device and network.
func CleanupOtherExtclients(ctx context.Context, extclient *models.ExtClient) error {
	extclients, err := GetNetworkExtClients(ctx, extclient.Network)
	if err != nil {
		return err
	}

	for _, extI := range extclients {
		if extI.ClientID != extclient.ClientID && extI.DeviceID == extclient.DeviceID && extI.OwnerID == extclient.OwnerID {
			err = DeleteExtClient(ctx, extI.Network, extI.ClientID, false)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
