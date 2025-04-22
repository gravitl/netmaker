package controller

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/exp/slog"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// swagger:route PUT /api/v1/nodes/migrate nodes migrateData
//
// Used to migrate a legacy node.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: hostPull
func migrate(w http.ResponseWriter, r *http.Request) {
	data := models.MigrationData{}
	host := models.Host{}
	node := models.Node{}
	nodes := []models.Node{}
	server := models.ServerConfig{}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	for i, legacy := range data.LegacyNodes {
		record, err := database.FetchRecord(database.NODES_TABLE_NAME, legacy.ID)
		if err != nil {
			slog.Error("legacy node not found", "error", err)
			logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("legacy node not found %w", err), "badrequest"))
			return
		}
		var legacyNode models.LegacyNode
		if err = json.Unmarshal([]byte(record), &legacyNode); err != nil {
			slog.Error("decoding legacy node", "error", err)
			logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("decode legacy node %w", err), "badrequest"))
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(legacyNode.Password), []byte(legacy.Password)); err != nil {
			slog.Error("legacy node invalid password", "error", err)
			logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("invalid password %w", err), "unauthorized"))
			return
		}
		if i == 0 {
			host, node = convertLegacyHostNode(legacy)
			host.Name = data.HostName
			host.HostPass = data.Password
			host.OS = data.OS
			if err := logic.CreateHost(&host); err != nil {
				slog.Error("create host", "error", err)
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
				return
			}
			server = logic.GetServerInfo()
			key, keyErr := logic.RetrievePublicTrafficKey()
			if keyErr != nil {
				slog.Error("retrieving traffickey", "error", err)
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
				return
			}
			server.TrafficKey = key
		} else {
			node = convertLegacyNode(legacyNode, host.ID)
		}
		if err := logic.UpsertNode(&node); err != nil {
			slog.Error("update node", "error", err)
			continue
		}
		host.Nodes = append(host.Nodes, node.ID.String())

		nodes = append(nodes, node)
	}
	if err := logic.UpsertHost(&host); err != nil {
		slog.Error("save host", "error", err)
	}
	go mq.PublishPeerUpdate(false)
	response := models.HostPull{
		Host:         host,
		Nodes:        nodes,
		ServerConfig: server,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&response)

	slog.Info("migrated nodes")
	// check for gateways
	for _, node := range data.LegacyNodes {
		if node.IsEgressGateway == "yes" {
			egressGateway := models.EgressGatewayRequest{
				NodeID:     node.ID,
				Ranges:     node.EgressGatewayRanges,
				NatEnabled: node.EgressGatewayNatEnabled,
			}
			if _, err := logic.CreateEgressGateway(egressGateway); err != nil {
				logger.Log(0, "error creating egress gateway for node", node.ID, err.Error())
			}
		}
		if node.IsIngressGateway == "yes" {
			ingressGateway := models.IngressRequest{}
			ingressNode, err := logic.CreateIngressGateway(node.Network, node.ID, ingressGateway)
			if err != nil {
				logger.Log(0, "error creating ingress gateway for node", node.ID, err.Error())
			}
			go func() {
				if err := mq.NodeUpdate(&ingressNode); err != nil {
					slog.Error("error publishing node update to node", "node", ingressNode.ID, "error", err)
				}
			}()
		}
	}
}

func convertLegacyHostNode(legacy models.LegacyNode) (models.Host, models.Node) {
	//convert host
	host := models.Host{}
	host.ID = uuid.New()
	host.IPForwarding = models.ParseBool(legacy.IPForwarding)
	host.AutoUpdate = logic.AutoUpdateEnabled()
	host.Interface = "netmaker"
	host.ListenPort = int(legacy.ListenPort)
	if host.ListenPort == 0 {
		host.ListenPort = 51821
	}
	host.MTU = int(legacy.MTU)
	host.PublicKey, _ = wgtypes.ParseKey(legacy.PublicKey)
	host.MacAddress = net.HardwareAddr(legacy.MacAddress)
	host.TrafficKeyPublic = legacy.TrafficKeys.Mine
	host.Nodes = append([]string{}, legacy.ID)
	host.Interfaces = legacy.Interfaces
	//host.DefaultInterface = legacy.Defaul
	host.EndpointIP = net.ParseIP(legacy.Endpoint)
	host.IsDocker = models.ParseBool(legacy.IsDocker)
	host.IsK8S = models.ParseBool(legacy.IsK8S)
	host.IsStaticPort = models.ParseBool(legacy.IsStatic)
	host.IsStatic = models.ParseBool(legacy.IsStatic)
	host.PersistentKeepalive = time.Duration(legacy.PersistentKeepalive) * time.Second
	if host.PersistentKeepalive == 0 {
		host.PersistentKeepalive = models.DefaultPersistentKeepAlive
	}

	node := convertLegacyNode(legacy, host.ID)
	return host, node
}

func convertLegacyNode(legacy models.LegacyNode, hostID uuid.UUID) models.Node {
	//convert node
	node := models.Node{}
	node.ID, _ = uuid.Parse(legacy.ID)
	node.HostID = hostID
	node.Network = legacy.Network
	valid4 := true
	valid6 := true
	_, cidr4, err := net.ParseCIDR(legacy.NetworkSettings.AddressRange)
	if err != nil {
		valid4 = false
		slog.Warn("parsing address range", "error", err)
	} else {
		node.NetworkRange = *cidr4
	}
	_, cidr6, err := net.ParseCIDR(legacy.NetworkSettings.AddressRange6)
	if err != nil {
		valid6 = false
		slog.Warn("parsing address range6", "error", err)
	} else {
		node.NetworkRange6 = *cidr6
	}
	node.Server = servercfg.GetServer()
	node.Connected = models.ParseBool(legacy.Connected)
	if valid4 {
		node.Address = net.IPNet{
			IP:   net.ParseIP(legacy.Address),
			Mask: cidr4.Mask,
		}
	}
	if valid6 {
		node.Address6 = net.IPNet{
			IP:   net.ParseIP(legacy.Address6),
			Mask: cidr6.Mask,
		}
	}
	node.Action = models.NODE_NOOP
	node.LocalAddress = net.IPNet{
		IP: net.ParseIP(legacy.LocalAddress),
	}
	node.IsEgressGateway = models.ParseBool(legacy.IsEgressGateway)
	node.EgressGatewayRanges = legacy.EgressGatewayRanges
	node.IsIngressGateway = models.ParseBool(legacy.IsIngressGateway)
	node.IsRelayed = false
	node.IsRelay = false
	node.RelayedNodes = []string{}
	node.DNSOn = models.ParseBool(legacy.DNSOn)
	node.LastModified = time.Now()
	node.ExpirationDateTime = time.Unix(legacy.ExpirationDateTime, 0)
	node.EgressGatewayNatEnabled = models.ParseBool(legacy.EgressGatewayNatEnabled)
	node.EgressGatewayRequest = legacy.EgressGatewayRequest
	node.IngressGatewayRange = legacy.IngressGatewayRange
	node.IngressGatewayRange6 = legacy.IngressGatewayRange6
	node.DefaultACL = legacy.DefaultACL
	node.OwnerID = legacy.OwnerID
	return node
}
