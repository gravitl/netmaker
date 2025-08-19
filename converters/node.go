package converters

import (
	"net"
	"sync"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func ToSchemaNode(node models.Node) schema.Node {
	var nodeID = node.ID.String()

	var networkRange, networkRange6 string
	if node.NetworkRange.IP != nil {
		networkRange = node.NetworkRange.String()
	}

	if node.NetworkRange6.IP != nil {
		networkRange6 = node.NetworkRange6.String()
	}

	var address, address6 string
	if node.Address.IP != nil {
		address = node.Address.String()
	}

	if node.Address6.IP != nil {
		address6 = node.Address6.String()
	}

	var gatewayNodeID *string
	var gatewayNode *schema.Node
	if node.IsRelayed {
		// assigning a separate variable to gatewayNodeID
		// to decouple it from the node object.
		relayNodeID := node.RelayedBy
		gatewayNodeID = &relayNodeID
		gatewayNode = &schema.Node{
			ID: relayNodeID,
		}
	}

	gatewayFor := make([]schema.Node, len(node.RelayedNodes))
	for i, relayedNodeID := range node.RelayedNodes {
		gatewayFor[i] = schema.Node{
			ID: relayedNodeID,
		}
	}

	var gatewayNodeConfig *datatypes.JSONType[schema.GatewayNodeConfig]
	if node.IsGw || node.IsRelay || node.IsIngressGateway {
		config := datatypes.NewJSONType(schema.GatewayNodeConfig{
			Range:               node.IngressGatewayRange,
			Range6:              node.IngressGatewayRange6,
			PersistentKeepalive: node.IngressPersistentKeepalive,
			MTU:                 node.IngressMTU,
			DNS:                 node.IngressDNS,
		})
		gatewayNodeConfig = &config
	}

	additionalGatewayIPs := make(datatypes.JSONSlice[string], len(node.AdditionalRagIps))
	for i, ip := range node.AdditionalRagIps {
		additionalGatewayIPs[i] = ip.String()
	}

	var failOverNodeID *string
	if node.FailedOverBy != uuid.Nil {
		// it's important we do this, because failOverNodeID
		// is a pointer and an empty string and a nil ptr
		// are different in db even though they both
		// represent the absence of a failOver.
		failOverID := node.FailedOverBy.String()
		failOverNodeID = &failOverID
	}

	failOverPeers := make(datatypes.JSONMap)
	for peer := range node.FailOverPeers {
		failOverPeers[peer] = true
	}

	var internetGatewayNodeID *string
	var internetGatewayNode *schema.Node
	if node.InternetGwID != "" && node.InternetGwID != uuid.Nil.String() {
		_internetGatewayNodeID := node.InternetGwID
		internetGatewayNodeID = &_internetGatewayNodeID
		internetGatewayNode = &schema.Node{
			ID: _internetGatewayNodeID,
		}
	}

	internetGatewayFor := make([]schema.Node, len(node.InetNodeReq.InetNodeClientIDs))
	if node.IsInternetGateway {
		for i, inetNodeClientID := range node.InetNodeReq.InetNodeClientIDs {
			internetGatewayFor[i] = schema.Node{
				ID: inetNodeClientID,
			}
		}
	}

	tags := make(datatypes.JSONMap)
	for tag := range node.Tags {
		tags[tag.String()] = struct{}{}
	}

	var _node = schema.Node{
		ID:                 nodeID,
		OwnerID:            node.OwnerID,
		HostID:             node.HostID.String(),
		LocalAddress:       node.LocalAddress.String(),
		NetworkID:          node.Network,
		NetworkRange:       networkRange,
		NetworkRange6:      networkRange6,
		Address:            address,
		Address6:           address6,
		Server:             node.Server,
		Connected:          node.Connected,
		Action:             node.Action,
		Status:             string(node.Status),
		DefaultACL:         node.DefaultACL,
		Metadata:           node.Metadata,
		Tags:               tags,
		PendingDelete:      node.PendingDelete,
		LastModified:       node.LastModified,
		LastCheckIn:        node.LastCheckIn,
		LastPeerUpdate:     node.LastPeerUpdate,
		ExpirationDateTime: node.ExpirationDateTime,
	}

	_node.GatewayNodeID = gatewayNodeID
	_node.GatewayNode = gatewayNode
	_node.GatewayFor = gatewayFor
	_node.GatewayNodeConfig = gatewayNodeConfig
	_node.AdditionalGatewayIPs = additionalGatewayIPs
	_node.FailOverNodeID = failOverNodeID
	_node.FailOverPeers = failOverPeers
	_node.IsFailOver = node.IsFailOver
	_node.InternetGatewayNodeID = internetGatewayNodeID
	_node.InternetGatewayNode = internetGatewayNode
	_node.InternetGatewayFor = internetGatewayFor
	_node.IsInternetGateway = node.IsInternetGateway

	return _node
}

func ToSchemaNodes(nodes []models.Node) []schema.Node {
	_nodes := make([]schema.Node, len(nodes))
	for i, node := range nodes {
		_nodes[i] = ToSchemaNode(node)
	}

	return _nodes
}

func ToModelNode(_node schema.Node) models.Node {
	var networkRange, networkRange6 net.IPNet
	if _node.NetworkRange != "" {
		_, _networkRange, _ := net.ParseCIDR(_node.NetworkRange)
		if _networkRange != nil {
			networkRange = *_networkRange
		}
	}

	if _node.NetworkRange6 != "" {
		_, _networkRange6, _ := net.ParseCIDR(_node.NetworkRange6)
		if _networkRange6 != nil {
			networkRange6 = *_networkRange6
		}
	}

	var address, address6 net.IPNet
	if _node.Address != "" {
		_ipv4, _address, _ := net.ParseCIDR(_node.Address)
		if _ipv4 != nil && _address != nil {
			address = *_address
			address.IP = _ipv4
		}
	}

	if _node.Address6 != "" {
		_ipv6, _address6, _ := net.ParseCIDR(_node.Address6)
		if _ipv6 != nil && _address6 != nil {
			address6 = *_address6
			address6.IP = _ipv6
		}
	}

	var localAddress net.IPNet
	if _node.LocalAddress != "" {
		_ip, _localAddress, _ := net.ParseCIDR(_node.LocalAddress)
		if _ip != nil && _localAddress != nil {
			localAddress = *_localAddress
			localAddress.IP = _ip
		}
	}

	additionalRagIPs := make([]net.IP, len(_node.AdditionalGatewayIPs))
	for _, ip := range _node.AdditionalGatewayIPs {
		additionalRagIPs = append(additionalRagIPs, net.ParseIP(ip))
	}

	var node = models.Node{
		CommonNode: models.CommonNode{
			ID:            uuid.MustParse(_node.ID),
			HostID:        uuid.MustParse(_node.HostID),
			Network:       _node.NetworkID,
			NetworkRange:  networkRange,
			NetworkRange6: networkRange6,
			Server:        _node.Server,
			Connected:     _node.Connected,
			Address:       address,
			Address6:      address6,
			Action:        _node.Action,
			LocalAddress:  localAddress,
		},
		PendingDelete:      _node.PendingDelete,
		LastModified:       _node.LastModified,
		LastCheckIn:        _node.LastCheckIn,
		LastPeerUpdate:     _node.LastPeerUpdate,
		ExpirationDateTime: _node.ExpirationDateTime,
		Metadata:           _node.Metadata,
		DefaultACL:         _node.DefaultACL,
		OwnerID:            _node.OwnerID,
		IsFailOver:         _node.IsFailOver,
		FailOverPeers:      make(map[string]struct{}),
		AdditionalRagIps:   additionalRagIPs,
		Tags:               make(map[models.TagID]struct{}),
		IsStatic:           false,
		IsUserNode:         false,
		Status:             models.NodeStatus(_node.Status),
		Mutex:              &sync.Mutex{},
	}

	if _node.GatewayNodeConfig != nil {
		node.IsGw = true
		node.IsIngressGateway = true
		node.IsRelay = true
		node.IngressGatewayRange = _node.GatewayNodeConfig.Data().Range
		node.IngressGatewayRange6 = _node.GatewayNodeConfig.Data().Range6
		node.IngressPersistentKeepalive = _node.GatewayNodeConfig.Data().PersistentKeepalive
		node.IngressMTU = _node.GatewayNodeConfig.Data().MTU
		node.IngressDNS = _node.GatewayNodeConfig.Data().DNS
		node.RelayedNodes = make([]string, len(_node.GatewayFor))
		for i, gatewayForNode := range _node.GatewayFor {
			node.RelayedNodes[i] = gatewayForNode.ID
		}
	}

	if _node.GatewayNodeID != nil {
		node.IsRelayed = true
		node.RelayedBy = *_node.GatewayNodeID
	} else if _node.GatewayNode != nil {
		node.IsRelayed = true
		node.RelayedBy = _node.GatewayNode.ID
	}

	if _node.FailOverNodeID != nil {
		node.FailedOverBy = uuid.MustParse(*_node.FailOverNodeID)
	}

	for peer := range _node.FailOverPeers {
		node.FailOverPeers[peer] = struct{}{}
	}

	for tag := range _node.Tags {
		node.Tags[models.TagID(tag)] = struct{}{}
	}

	if _node.IsInternetGateway {
		node.IsInternetGateway = true
		node.InetNodeReq.InetNodeClientIDs = make([]string, len(_node.InternetGatewayFor))
		for i, internetGatewayForNode := range _node.InternetGatewayFor {
			node.InetNodeReq.InetNodeClientIDs[i] = internetGatewayForNode.ID
		}
	}

	if _node.InternetGatewayNodeID != nil {
		node.InternetGwID = *_node.InternetGatewayNodeID
	} else if _node.InternetGatewayNode != nil {
		node.InternetGwID = _node.InternetGatewayNode.ID
	}

	return node
}

func ToModelNodes(_nodes []schema.Node) []models.Node {
	nodes := make([]models.Node, len(_nodes))
	for i, _node := range _nodes {
		nodes[i] = ToModelNode(_node)
	}

	return nodes
}
