package converters

import (
	"context"
	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
	"net"
	"sync"
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

	var egressGatewayNodeConfig *datatypes.JSONType[schema.EgressGatewayNodeConfig]
	if node.IsEgressGateway {
		var ranges []schema.RangeWithMetric
		for _, gwRange := range node.EgressGatewayRequest.RangesWithMetric {
			ranges = append(ranges, schema.RangeWithMetric{
				Range:  gwRange.Network,
				Metric: gwRange.RouteMetric,
			})
		}
		config := datatypes.NewJSONType(schema.EgressGatewayNodeConfig{
			NatEnabled: models.ParseBool(node.EgressGatewayRequest.NatEnabled),
			Ranges:     ranges,
		})
		egressGatewayNodeConfig = &config
	}

	failOverPeers := make(datatypes.JSONMap)
	if node.IsFailOver {
		for peer := range node.FailOverPeers {
			failOverPeers[peer] = true
		}
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

	var internetGatewayFor []schema.Node
	if node.IsInternetGateway {
		for _, internetGatewayForNode := range node.InetNodeReq.InetNodeClientIDs {
			internetGatewayFor = append(internetGatewayFor, schema.Node{
				ID: internetGatewayForNode,
			})
		}
	}

	var tags []string
	for tag := range node.Tags {
		tags = append(tags, string(tag))
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
	_node.GatewayNodeConfig = gatewayNodeConfig
	_node.EgressGatewayNodeConfig = egressGatewayNodeConfig
	_node.FailOverPeers = failOverPeers
	_node.InternetGatewayNodeID = internetGatewayNodeID
	_node.InternetGatewayNode = internetGatewayNode
	_node.InternetGatewayFor = internetGatewayFor
	_node.IsInternetGateway = node.IsInternetGateway

	// no information present about these in the models.Node
	// object.
	_node.GatewayFor = nil

	return _node
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

	var tags = make(map[models.TagID]struct{})
	for _, tag := range _node.Tags {
		tags[models.TagID(tag)] = struct{}{}
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
		IsFailOver:         false,
		FailOverPeers:      nil,
		FailedOverBy:       uuid.UUID{},
		Tags:               tags,
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

		for _, relayedNode := range _node.GatewayFor {
			node.RelayedNodes = append(node.RelayedNodes, relayedNode.ID)
		}
	}

	if _node.GatewayNodeID != nil && _node.GatewayNode != nil {
		node.IsRelayed = true
		node.RelayedBy = _node.GatewayNode.ID
	}

	if _node.EgressGatewayNodeConfig != nil {
		node.IsEgressGateway = true
		node.EgressGatewayRequest.NodeID = _node.ID
		node.EgressGatewayRequest.NetID = _node.NetworkID

		if _node.EgressGatewayNodeConfig.Data().NatEnabled {
			node.EgressGatewayRequest.NatEnabled = "yes"
		} else {
			node.EgressGatewayRequest.NatEnabled = "no"
		}

		for _, networkRange := range _node.EgressGatewayNodeConfig.Data().Ranges {
			node.EgressGatewayRequest.Ranges = append(node.EgressGatewayRequest.Ranges, networkRange.Range)
			node.EgressGatewayRequest.RangesWithMetric = append(node.EgressGatewayRequest.RangesWithMetric, models.EgressRangeMetric{
				Network:     networkRange.Range,
				RouteMetric: networkRange.Metric,
			})
		}

		node.EgressGatewayRanges = node.EgressGatewayRequest.Ranges
		node.EgressGatewayNatEnabled = _node.EgressGatewayNodeConfig.Data().NatEnabled
	}

	_network := &schema.Network{
		ID: _node.NetworkID,
	}
	err := _network.Get(db.WithContext(context.TODO()))
	if err == nil && _network.FailOverNodeID != nil {
		if *_network.FailOverNodeID == _node.ID {
			node.IsFailOver = true
		}

		if _node.FailOverPeers != nil {
			node.FailedOverBy = uuid.MustParse(*_network.FailOverNodeID)
			node.FailOverPeers = make(map[string]struct{})
			for peer := range _node.FailOverPeers {
				node.FailOverPeers[peer] = struct{}{}
			}
		}
	}

	if _node.IsInternetGateway {
		node.IsInternetGateway = true
		for _, internetGatewayForNode := range _node.InternetGatewayFor {
			node.InetNodeReq.InetNodeClientIDs = append(node.InetNodeReq.InetNodeClientIDs, internetGatewayForNode.ID)
		}
	}

	if _node.InternetGatewayNodeID != nil && _node.InternetGatewayNode != nil {
		node.InternetGwID = _node.InternetGatewayNode.ID
	}

	return node
}

func ToModelNodes(_nodes []schema.Node) []models.Node {
	var nodes []models.Node
	for _, _node := range _nodes {
		nodes = append(nodes, ToModelNode(_node))
	}

	return nodes
}
