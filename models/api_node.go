package models

import (
	"net"
	"time"

	"github.com/google/uuid"
)

// ApiNode is a stripped down Node DTO that exposes only required fields to external systems
type ApiNode struct {
	ID                      string   `json:"id,omitempty" validate:"required,min=5,id_unique"`
	HostID                  string   `json:"hostid,omitempty" validate:"required,min=5,id_unique"`
	Address                 string   `json:"address" validate:"omitempty,ipv4"`
	Address6                string   `json:"address6" validate:"omitempty,ipv6"`
	PostUp                  string   `json:"postup"`
	PostDown                string   `json:"postdown"`
	AllowedIPs              []string `json:"allowedips"`
	PersistentKeepalive     int32    `json:"persistentkeepalive"`
	LastModified            int64    `json:"lastmodified"`
	ExpirationDateTime      int64    `json:"expdatetime"`
	LastCheckIn             int64    `json:"lastcheckin"`
	LastPeerUpdate          int64    `json:"lastpeerupdate"`
	Network                 string   `json:"network"`
	NetworkRange            string   `json:"networkrange"`
	NetworkRange6           string   `json:"networkrange6"`
	IsRelayed               bool     `json:"isrelayed"`
	IsRelay                 bool     `json:"isrelay"`
	IsEgressGateway         bool     `json:"isegressgateway"`
	IsIngressGateway        bool     `json:"isingressgateway"`
	EgressGatewayRanges     []string `json:"egressgatewayranges"`
	EgressGatewayNatEnabled bool     `json:"egressgatewaynatenabled"`
	RelayAddrs              []string `json:"relayaddrs"`
	FailoverNode            string   `json:"failovernode"`
	DNSOn                   bool     `json:"dnson"`
	IsLocal                 bool     `json:"islocal"`
	Server                  string   `json:"server"`
	InternetGateway         string   `json:"internetgateway"`
	Connected               bool     `json:"connected"`
	PendingDelete           bool     `json:"pendingdelete"`
	// == PRO ==
	DefaultACL string `json:"defaultacl,omitempty" validate:"checkyesornoorunset"`
	Failover   bool   `json:"failover"`
}

// ApiNode.ConvertToServerNode - converts an api node to a server node
func (a *ApiNode) ConvertToServerNode(currentNode *Node) *Node {
	convertedNode := Node{}
	convertedNode.Network = a.Network
	convertedNode.Server = a.Server
	convertedNode.Action = currentNode.Action
	convertedNode.Connected = a.Connected
	convertedNode.AllowedIPs = a.AllowedIPs
	convertedNode.ID, _ = uuid.Parse(a.ID)
	convertedNode.HostID, _ = uuid.Parse(a.HostID)
	convertedNode.PostUp = a.PostUp
	convertedNode.PostDown = a.PostDown
	convertedNode.IsLocal = a.IsLocal
	convertedNode.IsRelay = a.IsRelay
	convertedNode.IsRelayed = a.IsRelayed
	convertedNode.PendingDelete = a.PendingDelete
	convertedNode.Peers = currentNode.Peers
	convertedNode.Failover = a.Failover
	convertedNode.IsEgressGateway = a.IsEgressGateway
	convertedNode.IsIngressGateway = a.IsIngressGateway
	convertedNode.EgressGatewayRanges = a.EgressGatewayRanges
	convertedNode.IngressGatewayRange = currentNode.IngressGatewayRange
	convertedNode.IngressGatewayRange6 = currentNode.IngressGatewayRange6
	convertedNode.DNSOn = a.DNSOn
	convertedNode.EgressGatewayRequest = currentNode.EgressGatewayRequest
	convertedNode.EgressGatewayNatEnabled = currentNode.EgressGatewayNatEnabled
	convertedNode.PersistentKeepalive = int(a.PersistentKeepalive)
	convertedNode.RelayAddrs = a.RelayAddrs
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
	udpAddr, err := net.ResolveUDPAddr("udp", a.InternetGateway)
	if err == nil {
		convertedNode.InternetGateway = udpAddr
	}
	_, addr, err := net.ParseCIDR(a.Address)
	if err == nil {
		convertedNode.Address = *addr
	}
	_, addr6, err := net.ParseCIDR(a.Address6)
	if err == nil {
		convertedNode.Address = *addr6
	}
	convertedNode.FailoverNode, _ = uuid.Parse(a.FailoverNode)
	convertedNode.LastModified = time.Unix(a.LastModified, 0)
	convertedNode.LastCheckIn = time.Unix(a.LastCheckIn, 0)
	convertedNode.LastPeerUpdate = time.Unix(a.LastPeerUpdate, 0)
	convertedNode.ExpirationDateTime = time.Unix(a.ExpirationDateTime, 0)
	return &convertedNode
}
