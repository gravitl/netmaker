package models

import (
	"net"
	"time"

	"github.com/google/uuid"
	"golang.org/x/exp/slog"
)

type ApiNodeStatus struct {
	ID         string     `json:"id"`
	IsStatic   bool       `json:"is_static"`
	IsUserNode bool       `json:"is_user_node"`
	Status     NodeStatus `json:"status"`
}

// ApiNode is a stripped down Node DTO that exposes only required fields to external systems
type ApiNode struct {
	ID                            string              `json:"id,omitempty" validate:"required,min=5,id_unique"`
	HostID                        string              `json:"hostid,omitempty" validate:"required,min=5,id_unique"`
	Address                       string              `json:"address" validate:"omitempty,cidrv4"`
	Address6                      string              `json:"address6" validate:"omitempty,cidrv6"`
	LocalAddress                  string              `json:"localaddress" validate:"omitempty,cidr"`
	AllowedIPs                    []string            `json:"allowedips"`
	LastModified                  int64               `json:"lastmodified" swaggertype:"primitive,integer" format:"int64"`
	ExpirationDateTime            int64               `json:"expdatetime" swaggertype:"primitive,integer" format:"int64"`
	LastCheckIn                   int64               `json:"lastcheckin" swaggertype:"primitive,integer" format:"int64"`
	LastPeerUpdate                int64               `json:"lastpeerupdate" swaggertype:"primitive,integer" format:"int64"`
	Network                       string              `json:"network"`
	NetworkRange                  string              `json:"networkrange"`
	NetworkRange6                 string              `json:"networkrange6"`
	IsRelayed                     bool                `json:"isrelayed"`
	IsRelay                       bool                `json:"isrelay"`
	RelayedBy                     string              `json:"relayedby" bson:"relayedby" yaml:"relayedby"`
	RelayedNodes                  []string            `json:"relaynodes" yaml:"relayedNodes"`
	IsEgressGateway               bool                `json:"isegressgateway"`
	IsIngressGateway              bool                `json:"isingressgateway"`
	EgressGatewayRanges           []string            `json:"egressgatewayranges"`
	EgressGatewayNatEnabled       bool                `json:"egressgatewaynatenabled"`
	EgressGatewayRangesWithMetric []EgressRangeMetric `json:"egressgatewayranges_with_metric"`
	DNSOn                         bool                `json:"dnson"`
	IngressDns                    string              `json:"ingressdns"`
	IngressPersistentKeepalive    int32               `json:"ingresspersistentkeepalive"`
	IngressMTU                    int32               `json:"ingressmtu"`
	Server                        string              `json:"server"`
	Connected                     bool                `json:"connected"`
	PendingDelete                 bool                `json:"pendingdelete"`
	Metadata                      string              `json:"metadata"`
	// == PRO ==
	DefaultACL        string              `json:"defaultacl,omitempty" validate:"checkyesornoorunset"`
	IsFailOver        bool                `json:"is_fail_over"`
	FailOverPeers     map[string]struct{} `json:"fail_over_peers" yaml:"fail_over_peers"`
	FailedOverBy      uuid.UUID           `json:"failed_over_by" yaml:"failed_over_by"`
	IsInternetGateway bool                `json:"isinternetgateway" yaml:"isinternetgateway"`
	InetNodeReq       InetNodeReq         `json:"inet_node_req" yaml:"inet_node_req"`
	InternetGwID      string              `json:"internetgw_node_id" yaml:"internetgw_node_id"`
	AdditionalRagIps  []string            `json:"additional_rag_ips" yaml:"additional_rag_ips"`
	Tags              map[TagID]struct{}  `json:"tags" yaml:"tags"`
	IsStatic          bool                `json:"is_static"`
	IsUserNode        bool                `json:"is_user_node"`
	StaticNode        ExtClient           `json:"static_node"`
	Status            NodeStatus          `json:"status"`
}

// ApiNode.ConvertToServerNode - converts an api node to a server node
func (a *ApiNode) ConvertToServerNode(currentNode *Node) *Node {
	convertedNode := Node{}
	convertedNode.Network = a.Network
	convertedNode.Server = a.Server
	convertedNode.Action = currentNode.Action
	convertedNode.Connected = a.Connected
	convertedNode.ID, _ = uuid.Parse(a.ID)
	convertedNode.HostID, _ = uuid.Parse(a.HostID)
	convertedNode.IsRelay = a.IsRelay
	convertedNode.IsRelayed = a.IsRelayed
	convertedNode.RelayedBy = a.RelayedBy
	convertedNode.RelayedNodes = a.RelayedNodes
	convertedNode.PendingDelete = a.PendingDelete
	convertedNode.FailedOverBy = currentNode.FailedOverBy
	convertedNode.FailOverPeers = currentNode.FailOverPeers
	convertedNode.IsEgressGateway = a.IsEgressGateway
	convertedNode.IsIngressGateway = a.IsIngressGateway
	// prevents user from changing ranges, must delete and recreate
	convertedNode.EgressGatewayRanges = currentNode.EgressGatewayRanges
	convertedNode.IngressGatewayRange = currentNode.IngressGatewayRange
	convertedNode.IngressGatewayRange6 = currentNode.IngressGatewayRange6
	convertedNode.DNSOn = a.DNSOn
	convertedNode.IngressDNS = a.IngressDns
	convertedNode.IngressPersistentKeepalive = a.IngressPersistentKeepalive
	convertedNode.IngressMTU = a.IngressMTU
	convertedNode.IsInternetGateway = a.IsInternetGateway
	convertedNode.EgressGatewayRequest = currentNode.EgressGatewayRequest
	convertedNode.EgressGatewayNatEnabled = currentNode.EgressGatewayNatEnabled
	convertedNode.InternetGwID = currentNode.InternetGwID
	convertedNode.InetNodeReq = currentNode.InetNodeReq
	convertedNode.RelayedNodes = a.RelayedNodes
	convertedNode.DefaultACL = a.DefaultACL
	convertedNode.OwnerID = currentNode.OwnerID
	_, networkRange, err := net.ParseCIDR(a.NetworkRange)
	if err == nil {
		convertedNode.NetworkRange = *networkRange
	}
	_, networkRange6, err := net.ParseCIDR(a.NetworkRange6)
	if err == nil {
		convertedNode.NetworkRange6 = *networkRange6
	}
	if len(a.LocalAddress) > 0 {
		_, localAddr, err := net.ParseCIDR(a.LocalAddress)
		if err == nil {
			convertedNode.LocalAddress = *localAddr
		}
	} else if !isEmptyAddr(currentNode.LocalAddress.String()) {
		convertedNode.LocalAddress = currentNode.LocalAddress
	}
	ip, addr, err := net.ParseCIDR(a.Address)
	if err == nil {
		convertedNode.Address = *addr
		convertedNode.Address.IP = ip
	}
	ip6, addr6, err := net.ParseCIDR(a.Address6)
	if err == nil {
		convertedNode.Address6 = *addr6
		convertedNode.Address6.IP = ip6
	}
	convertedNode.LastModified = time.Unix(a.LastModified, 0)
	convertedNode.LastCheckIn = time.Unix(a.LastCheckIn, 0)
	convertedNode.LastPeerUpdate = time.Unix(a.LastPeerUpdate, 0)
	convertedNode.ExpirationDateTime = time.Unix(a.ExpirationDateTime, 0)
	convertedNode.Metadata = a.Metadata
	for _, ip := range a.AdditionalRagIps {
		ragIp := net.ParseIP(ip)
		if ragIp == nil {
			slog.Error("error parsing additional rag ip", "error", err, "ip", ip)
			return nil
		}
		convertedNode.AdditionalRagIps = append(convertedNode.AdditionalRagIps, ragIp)
	}
	convertedNode.Tags = a.Tags
	return &convertedNode
}

func (nm *Node) ConvertToStatusNode() *ApiNodeStatus {
	apiNode := ApiNodeStatus{}
	if nm.IsStatic {
		apiNode.ID = nm.StaticNode.ClientID
	} else {
		apiNode.ID = nm.ID.String()
	}
	apiNode.IsStatic = nm.IsStatic
	apiNode.IsUserNode = nm.IsUserNode
	apiNode.Status = nm.Status
	return &apiNode
}

// Node.ConvertToAPINode - converts a node to an API node
func (nm *Node) ConvertToAPINode() *ApiNode {
	apiNode := ApiNode{}
	apiNode.ID = nm.ID.String()
	apiNode.HostID = nm.HostID.String()
	apiNode.Address = nm.Address.String()
	if isEmptyAddr(apiNode.Address) {
		apiNode.Address = ""
	}
	apiNode.Address6 = nm.Address6.String()
	if isEmptyAddr(apiNode.Address6) {
		apiNode.Address6 = ""
	}
	apiNode.LocalAddress = nm.LocalAddress.String()
	if isEmptyAddr(apiNode.LocalAddress) {
		apiNode.LocalAddress = ""
	}
	apiNode.LastModified = nm.LastModified.Unix()
	apiNode.LastCheckIn = nm.LastCheckIn.Unix()
	apiNode.LastPeerUpdate = nm.LastPeerUpdate.Unix()
	apiNode.ExpirationDateTime = nm.ExpirationDateTime.Unix()
	apiNode.Network = nm.Network
	apiNode.NetworkRange = nm.NetworkRange.String()
	if isEmptyAddr(apiNode.NetworkRange) {
		apiNode.NetworkRange = ""
	}
	apiNode.NetworkRange6 = nm.NetworkRange6.String()
	if isEmptyAddr(apiNode.NetworkRange6) {
		apiNode.NetworkRange6 = ""
	}
	apiNode.IsRelayed = nm.IsRelayed
	apiNode.IsRelay = nm.IsRelay
	apiNode.RelayedBy = nm.RelayedBy
	apiNode.RelayedNodes = nm.RelayedNodes
	apiNode.IsEgressGateway = nm.IsEgressGateway
	apiNode.IsIngressGateway = nm.IsIngressGateway
	apiNode.EgressGatewayRanges = nm.EgressGatewayRanges
	apiNode.EgressGatewayRangesWithMetric = nm.EgressGatewayRequest.RangesWithMetric
	apiNode.EgressGatewayNatEnabled = nm.EgressGatewayNatEnabled
	apiNode.DNSOn = nm.DNSOn
	apiNode.IngressDns = nm.IngressDNS
	apiNode.IngressPersistentKeepalive = nm.IngressPersistentKeepalive
	apiNode.IngressMTU = nm.IngressMTU
	apiNode.Server = nm.Server
	apiNode.Connected = nm.Connected
	apiNode.PendingDelete = nm.PendingDelete
	apiNode.DefaultACL = nm.DefaultACL
	apiNode.IsInternetGateway = nm.IsInternetGateway
	apiNode.InternetGwID = nm.InternetGwID
	apiNode.InetNodeReq = nm.InetNodeReq
	apiNode.IsFailOver = nm.IsFailOver
	apiNode.FailOverPeers = nm.FailOverPeers
	apiNode.FailedOverBy = nm.FailedOverBy
	apiNode.Metadata = nm.Metadata
	apiNode.AdditionalRagIps = []string{}
	apiNode.Tags = nm.Tags
	for _, ip := range nm.AdditionalRagIps {
		apiNode.AdditionalRagIps = append(apiNode.AdditionalRagIps, ip.String())
	}
	apiNode.IsStatic = nm.IsStatic
	apiNode.IsUserNode = nm.IsUserNode
	apiNode.StaticNode = nm.StaticNode
	apiNode.Status = nm.Status
	return &apiNode
}

func isEmptyAddr(addr string) bool {
	return addr == "<nil>" || addr == ":0"
}
