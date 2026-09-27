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
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"golang.org/x/exp/slog"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var ErrClientLimitExceeded = errors.New("client limit reached for this tenant, please upgrade your license")

var ClientLimitExceeded = func(ctx context.Context) bool {
	return false
}

// ExtClient.GetEgressRangesOnNetwork - returns the egress ranges on network of ext client.
// Internet egress (0.0.0.0/0, ::/0) is excluded here: full-tunnel is opt-in via
// SelectedInternetEgressID and is applied only by ExtClientUsesInternetEgress /
// GetExtclientAllowedIPs (and the matching config-file path).
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
		// Full-tunnel exit must not appear as a normal egress range in AllowedIPs.
		if IsEgressInternetGateway(eI) {
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
				err := user.GetWithMembership(ctx)
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

// DeleteExtClient deletes an existing ext client, its posture check
// violations and its references in acl policies.
func DeleteExtClient(ctx context.Context, extClient models.ExtClient) error {
	_extClient := &schema.Extclient{ID: extClient.ID}
	if err := _extClient.Delete(ctx); err != nil {
		return err
	}
	if err := _extClient.DeleteViolations(ctx); err != nil {
		slog.Error("failed to delete ext client posture check violations", "id", extClient.ID, "error", err)
	}
	if extClient.RemoteAccessClientID != "" {
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

//TODO - enforce extclient-to-extclient on ingress gw
/* 1. fetch all non-user static nodes
a. check against each user node, if allowed add rule

*/

// GetNetworkExtClients - gets the ext clients of given network
func GetNetworkExtClients(ctx context.Context, network string) ([]models.ExtClient, error) {
	_extclients, err := (&schema.Extclient{Network: &schema.Network{Name: network}}).ListByNetwork(ctx)
	if err != nil {
		return nil, err
	}

	extclients := make([]models.ExtClient, 0, len(_extclients))
	for i := range _extclients {
		extclients = append(extclients, *ConvertSchemaExtclientToModelsExtClientWithContext(ctx, &_extclients[i], SkipViolations()))
	}
	return extclients, nil
}

// GetExtClient - gets a single ext client on a network
func GetExtClient(ctx context.Context, clientid string, network string) (models.ExtClient, error) {
	_extclient := &schema.Extclient{
		Name:    clientid,
		Network: &schema.Network{Name: network},
	}
	if err := _extclient.Get(ctx); err != nil {
		return models.ExtClient{}, err
	}
	return *ConvertSchemaExtclientToModelsExtClientWithContext(ctx, _extclient), nil
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

// SaveExtClient creates the ext client if it has no ID, else updates it.
// Posture check violations are not saved, use Extclient.UpsertViolations.
func SaveExtClient(ctx context.Context, extclient *models.ExtClient) error {
	if extclient.TenantID == "" {
		extclient.TenantID = scope.ID(ctx)
	}
	_extclient, err := ConvertModelsExtClientToSchemaExtclient(extclient)
	if err != nil {
		return err
	}
	if extclient.ID == "" {
		if err = _extclient.Create(ctx); err != nil {
			return err
		}
		extclient.ID = _extclient.ID
	} else if err = _extclient.Update(ctx); err != nil {
		return err
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

// GetGatewayExtClients - gets the ext clients attached to the gateway
func GetGatewayExtClients(ctx context.Context, gatewayID string) ([]models.ExtClient, error) {
	return listExtClients(ctx, dbtypes.WithFilter(fmt.Sprintf("%s.ingress_gateway_id", (&schema.Extclient{}).TableName()), gatewayID))
}

// GetAllExtClients - gets all ext clients from DB
func GetAllExtClients(ctx context.Context) ([]models.ExtClient, error) {
	return listExtClients(ctx)
}

// GetAllExtClientsWithStatus - returns all external clients with
// given status.
func GetAllExtClientsWithStatus(ctx context.Context, status schema.NodeStatus) ([]models.ExtClient, error) {
	return listExtClients(ctx, dbtypes.WithFilter(fmt.Sprintf("%s.status", (&schema.Extclient{}).TableName()), status))
}

// listExtClients lists the ext clients of the tenant in the context matching
// the options, without their posture check violations.
func listExtClients(ctx context.Context, options ...dbtypes.Option) ([]models.ExtClient, error) {
	// the inner join skips the ext clients of deleted networks, and populates
	// the network of the rest for the conversion.
	options = append(options, func(db *gorm.DB) *gorm.DB {
		return db.InnerJoins("Network")
	})
	_extclients, err := (&schema.Extclient{}).ListAll(ctx, options...)
	if err != nil {
		return nil, err
	}

	extclients := make([]models.ExtClient, 0, len(_extclients))
	for i := range _extclients {
		extclients = append(extclients, *ConvertSchemaExtclientToModelsExtClientWithContext(ctx, &_extclients[i], SkipViolations()))
	}
	return extclients, nil
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
	if err := SaveExtClient(ctx, &newClient); err != nil {
		slog.Error("failed to save updated ext client during update", "id", newClient.ClientID, "network", newClient.Network, "error", err)
		return newClient, err
	}

	return newClient, nil
}

func GetExtPeers(ctx context.Context, node, peer *models.Node, addressIdentityMap map[string]models.PeerIdentity) ([]wgtypes.PeerConfig, []models.IDandAddr, []models.EgressNetworkRoutes, error) {
	var skipFlowLogs bool
	if !GetFeatureFlags(ctx).EnableFlowLogs || !GetServerSettings(ctx).EnableFlowLogs {
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
	if ExtClientUsesInternetEgress(client, gwnode) {
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
			err = DeleteExtClient(ctx, extI)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func ConvertSchemaExtclientToModelsExtClient(_extclient *schema.Extclient, opts ...NodeConvertOption) *models.ExtClient {
	ctx := scope.WithContext(db.WithContext(context.TODO()), scope.TenantScope, _extclient.TenantID)
	return ConvertSchemaExtclientToModelsExtClientWithContext(ctx, _extclient, opts...)
}

func ConvertSchemaExtclientToModelsExtClientWithContext(ctx context.Context, _extclient *schema.Extclient, opts ...NodeConvertOption) *models.ExtClient {
	cfg := nodeConvertOpts{}
	for _, opt := range opts {
		opt(&cfg)
	}

	if _extclient.Network == nil {
		_extclient.Network = &schema.Network{
			ID: _extclient.NetworkID,
		}
		err := _extclient.Network.Get(ctx)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				_extclient.Network = &schema.Network{}
			} else {
				return &models.ExtClient{}
			}
		}
	}

	var violations []models.Violation
	if !cfg.skipViolations {
		_violations, err := _extclient.ListViolations(ctx)
		if err == nil {
			for _, _violation := range _violations {
				violations = append(violations, models.Violation{
					CheckID:   _violation.CheckID,
					Name:      _violation.Name,
					Attribute: _violation.Attribute,
					Message:   _violation.Message,
					Severity:  _violation.Severity,
				})
			}
		}
	}

	return &models.ExtClient{
		ID:                                _extclient.ID,
		TenantID:                          _extclient.TenantID,
		ClientID:                          _extclient.Name,
		PrivateKey:                        _extclient.PrivateKey,
		PublicKey:                         _extclient.PublicKey,
		Network:                           _extclient.Network.Name,
		DNS:                               _extclient.DNS,
		Address:                           _extclient.Address,
		Address6:                          _extclient.Address6,
		ExtraAllowedIPs:                   _extclient.ExtraAllowedIPs,
		AllowedIPs:                        _extclient.AllowedIPs,
		IngressGatewayID:                  _extclient.IngressGatewayID,
		IngressGatewayEndpoint:            _extclient.IngressGatewayEndpoint,
		SelectedInternetEgressID:          _extclient.SelectedInternetEgressID,
		LastModified:                      _extclient.UpdatedAt.Unix(),
		Enabled:                           _extclient.Enabled,
		OwnerID:                           _extclient.OwnerID,
		DeniedACLs:                        _extclient.DeniedACLs.Data(),
		RemoteAccessClientID:              _extclient.RemoteAccessClientID,
		PostUp:                            _extclient.PostUp,
		PostDown:                          _extclient.PostDown,
		Tags:                              _extclient.Tags.Data(),
		OS:                                _extclient.OS,
		OSFamily:                          _extclient.OSFamily,
		OSVersion:                         _extclient.OSVersion,
		KernelVersion:                     _extclient.KernelVersion,
		ClientVersion:                     _extclient.ClientVersion,
		DeviceID:                          _extclient.DeviceID,
		DeviceName:                        _extclient.DeviceName,
		PublicEndpoint:                    _extclient.PublicEndpoint,
		Country:                           _extclient.Country,
		Location:                          _extclient.Location,
		PostureChecksViolations:           violations,
		PostureCheckVolationSeverityLevel: _extclient.PostureCheckSeverity,
		LastEvaluationCycleID:             _extclient.PostureCheckLastEvaluationCycleID,
		LastEvaluatedAt:                   _extclient.PostureCheckLastEvaluatedAt,
		JITExpiresAt:                      _extclient.JITExpiresAt,
		Status:                            _extclient.Status,
		Mutex:                             &sync.Mutex{},
	}
}

// ConvertModelsExtClientToSchemaExtclient converts the ext client, resolving
// its network by name within its tenant. Posture check violations are not
// converted.
func ConvertModelsExtClientToSchemaExtclient(extclient *models.ExtClient) (*schema.Extclient, error) {
	ctx := scope.WithContext(db.WithContext(context.TODO()), scope.TenantScope, extclient.TenantID)

	network := &schema.Network{
		Name: extclient.Network,
	}
	err := network.Get(ctx)
	if err != nil {
		return nil, err
	}

	return &schema.Extclient{
		ID:                                extclient.ID,
		TenantID:                          extclient.TenantID,
		NetworkID:                         network.ID,
		Network:                           network,
		Name:                              extclient.ClientID,
		PrivateKey:                        extclient.PrivateKey,
		PublicKey:                         extclient.PublicKey,
		DNS:                               extclient.DNS,
		Address:                           extclient.Address,
		Address6:                          extclient.Address6,
		IngressGatewayID:                  extclient.IngressGatewayID,
		IngressGatewayEndpoint:            extclient.IngressGatewayEndpoint,
		AllowedIPs:                        extclient.AllowedIPs,
		ExtraAllowedIPs:                   extclient.ExtraAllowedIPs,
		PostUp:                            extclient.PostUp,
		PostDown:                          extclient.PostDown,
		SelectedInternetEgressID:          extclient.SelectedInternetEgressID,
		Enabled:                           extclient.Enabled,
		OwnerID:                           extclient.OwnerID,
		Status:                            extclient.Status,
		PostureCheckSeverity:              extclient.PostureCheckVolationSeverityLevel,
		PostureCheckLastEvaluationCycleID: extclient.LastEvaluationCycleID,
		PostureCheckLastEvaluatedAt:       extclient.LastEvaluatedAt,
		DeniedACLs:                        datatypes.NewJSONType(extclient.DeniedACLs),
		RemoteAccessClientID:              extclient.RemoteAccessClientID,
		Tags:                              datatypes.NewJSONType(extclient.Tags),
		OS:                                extclient.OS,
		OSFamily:                          extclient.OSFamily,
		OSVersion:                         extclient.OSVersion,
		KernelVersion:                     extclient.KernelVersion,
		ClientVersion:                     extclient.ClientVersion,
		DeviceID:                          extclient.DeviceID,
		DeviceName:                        extclient.DeviceName,
		PublicEndpoint:                    extclient.PublicEndpoint,
		Country:                           extclient.Country,
		Location:                          extclient.Location,
		JITExpiresAt:                      extclient.JITExpiresAt,
	}, nil
}
