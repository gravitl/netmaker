package models

import (
	"bytes"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

// LastCheckInThreshold - if node's checkin more than this threshold,then node is declared as offline
const LastCheckInThreshold = time.Minute * 10

// NodeCheckin - struct for node checkins with server
type NodeCheckin struct {
	Version   string
	Connected bool
	Ifaces    []schema.Iface
}

// CommonNode - represents a commonn node data elements shared by netmaker and netclient
type CommonNode struct {
	ID                  uuid.UUID `json:"id"                  yaml:"id"`
	HostID              uuid.UUID `json:"hostid"              yaml:"hostid"`
	Network             string    `json:"network"             yaml:"network"`
	NetworkRange        net.IPNet `json:"networkrange"        yaml:"networkrange"        swaggertype:"primitive,integer"`
	NetworkRange6       net.IPNet `json:"networkrange6"       yaml:"networkrange6"       swaggertype:"primitive,number"`
	Server              string    `json:"server"              yaml:"server"`
	Connected           bool      `json:"connected"           yaml:"connected"`
	Address             net.IPNet `json:"address"             yaml:"address"`
	Address6            net.IPNet `json:"address6"            yaml:"address6"`
	Action              string    `json:"action"              yaml:"action"`
	LocalAddress        net.IPNet `json:"localaddress"        yaml:"localaddress"`
	IsEgressGateway     bool      `json:"isegressgateway"     yaml:"isegressgateway"`
	EgressGatewayRanges []string  `json:"egressgatewayranges" yaml:"egressgatewayranges"`
	IsIngressGateway    bool      `json:"isingressgateway"    yaml:"isingressgateway"`
	IsRelayed           bool      `json:"isrelayed"           yaml:"isrelayed"`
	RelayedBy           string    `json:"relayedby"           yaml:"relayedby"`
	IsRelay             bool      `json:"isrelay"             yaml:"isrelay"`
	IsGw                bool      `json:"is_gw"             yaml:"is_gw"`
	RelayedNodes        []string  `json:"relaynodes"          yaml:"relayedNodes"`
	IngressDNS          string    `json:"ingressdns"          yaml:"ingressdns"`
	AutoAssignGateway   bool      `json:"auto_assign_gw"`
}

// Node - a model of a network node
type Node struct {
	CommonNode
	PendingDelete              bool                 `json:"pendingdelete"`
	LastModified               time.Time            `json:"lastmodified"`
	LastCheckIn                time.Time            `json:"lastcheckin"`
	LastPeerUpdate             time.Time            `json:"lastpeerupdate"`
	ExpirationDateTime         time.Time            `json:"expdatetime"`
	EgressGatewayNatEnabled    bool                 `json:"egressgatewaynatenabled"`
	EgressGatewayRequest       EgressGatewayRequest `json:"egressgatewayrequest"`
	IngressGatewayRange        string               `json:"ingressgatewayrange"`
	IngressGatewayRange6       string               `json:"ingressgatewayrange6"`
	IngressPersistentKeepalive int32                `json:"ingresspersistentkeepalive"`
	IngressMTU                 int32                `json:"ingressmtu"`
	Metadata                   string               `json:"metadata"`
	// == PRO ==
	OwnerID     string `json:"ownerid,omitempty"`
	IsFailOver  bool   `json:"is_fail_over"`
	IsAutoRelay bool   `json:"is_auto_relay"`
	//AutoRelayedPeers   map[string]struct{} `json:"auto_relayed_peers"`
	AutoRelayedPeers map[string]string `json:"auto_relayed_peers_v1"`
	//AutoRelayedBy     uuid.UUID           `json:"auto_relayed_by"`
	FailOverPeers                     map[string]struct{} `json:"fail_over_peers"`
	FailedOverBy                      uuid.UUID           `json:"failed_over_by"`
	IsInternetGateway                 bool                `json:"isinternetgateway"`
	InetNodeReq                       InetNodeReq         `json:"inet_node_req"`
	InternetGwID                      string              `json:"internetgw_node_id"`
	AdditionalRagIps                  []net.IP            `json:"additional_rag_ips" swaggertype:"array,number"`
	Tags                              map[TagID]struct{}  `json:"tags"`
	IsStatic                          bool                `json:"is_static"`
	IsUserNode                        bool                `json:"is_user_node"`
	StaticNode                        ExtClient           `json:"static_node"`
	Status                            schema.NodeStatus   `json:"node_status"`
	Mutex                             *sync.Mutex         `json:"-"`
	EgressDetails                     EgressDetails       `json:"-"`
	PostureChecksViolations           []Violation         `json:"posture_check_violations"`
	PostureCheckVolationSeverityLevel schema.Severity     `json:"posture_check_violation_severity_level"`
	LastEvaluatedAt                   time.Time           `json:"last_evaluated_at"`
	Location                          string              `json:"location"` // Format: "lat,lon"
	CountryCode                       string              `json:"country_code"`
}
type EgressDetails struct {
	EgressGatewayNatEnabled bool
	EgressGatewayRequest    EgressGatewayRequest
	IsEgressGateway         bool
	EgressGatewayRanges     []string
	// IsInternetGateway       bool        `json:"isinternetgateway"                                      yaml:"isinternetgateway"`
	// InetNodeReq             InetNodeReq `json:"inet_node_req"                                          yaml:"inet_node_req"`
	// InternetGwID            string      `json:"internetgw_node_id"                                     yaml:"internetgw_node_id"`
}

// NodesArray - used for node sorting
type NodesArray []Node

// NodesArray.Len - gets length of node array
func (a NodesArray) Len() int { return len(a) }

// NodesArray.Less - gets returns lower rank of two node addressesFill
func (a NodesArray) Less(i, j int) bool {
	return isLess(a[i].Address.IP.String(), a[j].Address.IP.String())
}

// NodesArray.Swap - swaps two nodes in array
func (a NodesArray) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

func isLess(ipA string, ipB string) bool {
	ipNetA := net.ParseIP(ipA)
	ipNetB := net.ParseIP(ipB)
	return bytes.Compare(ipNetA, ipNetB) < 0
}

// Node.PrimaryAddress - return ipv4 address if present, else return ipv6
func (node *Node) PrimaryAddressIPNet() net.IPNet {
	if node.Address.IP != nil {
		return node.Address
	}
	return node.Address6
}

// Node.PrimaryAddress - return ipv4 address if present, else return ipv6
func (node *Node) PrimaryAddress() string {
	if node.Address.IP != nil {
		return node.Address.IP.String()
	}
	return node.Address6.IP.String()
}

func (node *Node) AddressIPNet4() net.IPNet {
	return net.IPNet{
		IP:   node.Address.IP,
		Mask: net.CIDRMask(32, 32),
	}
}
func (node *Node) AddressIPNet6() net.IPNet {
	return net.IPNet{
		IP:   node.Address6.IP,
		Mask: net.CIDRMask(128, 128),
	}
}

// ExtClient.PrimaryAddress - returns ipv4 IPNet format
func (extPeer *ExtClient) AddressIPNet4() net.IPNet {
	return net.IPNet{
		IP:   net.ParseIP(extPeer.Address),
		Mask: net.CIDRMask(32, 32),
	}
}

// ExtClient.AddressIPNet6 - return ipv6 IPNet format
func (extPeer *ExtClient) AddressIPNet6() net.IPNet {
	return net.IPNet{
		IP:   net.ParseIP(extPeer.Address6),
		Mask: net.CIDRMask(128, 128),
	}
}

// Node.PrimaryNetworkRange - returns node's parent network, returns ipv4 address if present, else return ipv6
func (node *Node) PrimaryNetworkRange() net.IPNet {
	if node.NetworkRange.IP != nil {
		return node.NetworkRange
	}
	return node.NetworkRange6
}

// Node.SetDefaultConnected
func (node *Node) SetDefaultConnected() {
	node.Connected = true
}

// Node.SetLastModified - set last modified initial time
func (node *Node) SetLastModified() {
	node.LastModified = time.Now().UTC()
}

// Node.SetLastCheckIn - set checkin time of node
func (node *Node) SetLastCheckIn() {
	node.LastCheckIn = time.Now().UTC()
}

// Node.SetLastPeerUpdate - sets last peer update time
func (node *Node) SetLastPeerUpdate() {
	node.LastPeerUpdate = time.Now().UTC()
}

// Node.SetExpirationDateTime - sets node expiry time
func (node *Node) SetExpirationDateTime() {
	if node.ExpirationDateTime.IsZero() {
		node.ExpirationDateTime = time.Now().AddDate(100, 1, 0)
	}
}

// Node.Fill - fills other node data into calling node data if not set on calling node (skips DNSOn)
func (newNode *Node) Fill(
	currentNode *Node,
	isPro bool,
) { // TODO add new field for nftables present
	newNode.ID = currentNode.ID
	newNode.HostID = currentNode.HostID
	// Revisit the logic for boolean values
	// TODO ---- !!!!!!!!!!!!!!!!!!!!!!!!!!!!
	// TODO ---- !!!!!!!!!!!!!!!!!!!!!!!!!!
	if newNode.Address.String() == "" {
		newNode.Address = currentNode.Address
	}
	if newNode.Address6.String() == "" {
		newNode.Address6 = currentNode.Address6
	}
	if newNode.LastModified != currentNode.LastModified {
		newNode.LastModified = currentNode.LastModified
	}
	if newNode.ExpirationDateTime.IsZero() {
		newNode.ExpirationDateTime = currentNode.ExpirationDateTime
	}
	if newNode.LastPeerUpdate.IsZero() || currentNode.LastPeerUpdate.After(newNode.LastPeerUpdate) {
		newNode.LastPeerUpdate = currentNode.LastPeerUpdate
	}
	if newNode.LastCheckIn.IsZero() || currentNode.LastCheckIn.After(newNode.LastCheckIn) {
		newNode.LastCheckIn = currentNode.LastCheckIn
	}
	if newNode.Network == "" {
		newNode.Network = currentNode.Network
	}
	if newNode.IsIngressGateway != currentNode.IsIngressGateway {
		newNode.IsIngressGateway = currentNode.IsIngressGateway
	}
	if newNode.IngressGatewayRange == "" {
		newNode.IngressGatewayRange = currentNode.IngressGatewayRange
	}
	if newNode.IngressGatewayRange6 == "" {
		newNode.IngressGatewayRange6 = currentNode.IngressGatewayRange6
	}
	if newNode.Action == "" {
		newNode.Action = currentNode.Action
	}
	if newNode.RelayedNodes == nil {
		newNode.RelayedNodes = currentNode.RelayedNodes
	}
	if newNode.IsRelay != currentNode.IsRelay && isPro {
		newNode.IsRelay = currentNode.IsRelay
	}
	if newNode.IsRelayed == currentNode.IsRelayed && isPro {
		newNode.IsRelayed = currentNode.IsRelayed
	}
	if newNode.Server == "" {
		newNode.Server = currentNode.Server
	}
	if newNode.IsFailOver != currentNode.IsFailOver {
		newNode.IsFailOver = currentNode.IsFailOver
	}
	newNode.FailOverPeers = currentNode.FailOverPeers
	if newNode.Tags == nil {
		if currentNode.Tags == nil {
			currentNode.Tags = make(map[TagID]struct{})
		}
		newNode.Tags = currentNode.Tags
	}
}

// Node.NetworkSettings updates a node with network settings
func (node *Node) NetworkSettings(n Network) {
	_, cidr, err := net.ParseCIDR(n.AddressRange)
	if err == nil {
		node.NetworkRange = *cidr
	}
	_, cidr, err = net.ParseCIDR(n.AddressRange6)
	if err == nil {
		node.NetworkRange6 = *cidr
	}
}

type NodeWithHost struct {
	ID                                string                                `json:"id"`
	HostID                            string                                `json:"host_id"`
	Host                              *ApiHost                              `json:"host,omitempty"`
	NetworkID                         string                                `json:"network_id"`
	Address                           string                                `json:"address"`
	Address6                          string                                `json:"address6"`
	Connected                         bool                                  `json:"connected"`
	Action                            string                                `json:"action"`
	Status                            schema.NodeStatus                     `json:"status"`
	PendingDelete                     bool                                  `json:"pending_delete"`
	AutoAssignGateway                 bool                                  `json:"auto_assign_gateway"`
	IsGateway                         bool                                  `json:"is_gateway"`
	IsAutoRelay                       bool                                  `json:"is_auto_relay"`
	IsInternetGateway                 bool                                  `json:"is_internet_gateway"`
	RelayedClients                    datatypes.JSONMap                     `json:"relayed_clients"`
	RelayedIGWClients                 datatypes.JSONMap                     `json:"relayed_igw_clients"`
	RelayedByNodeID                   *string                               `json:"relayed_by_node_id"`
	IsIGWClient                       bool                                  `json:"is_igw_client"`
	AutoRelayedPeers                  datatypes.JSONType[map[string]string] `json:"auto_relayed_peers"`
	Tags                              datatypes.JSONMap                     `json:"tags"`
	PostureCheckSeverity              schema.Severity                       `json:"posture_check_severity"`
	PostureCheckLastEvaluationCycleID string                                `json:"posture_check_last_evaluation_cycle_id"`
	Metadata                          string                                `json:"metadata"`
	LastCheckIn                       time.Time                             `json:"last_check_in"`
	ExpirationDateTime                time.Time                             `json:"expiration_date_time"`
	CreatedAt                         time.Time                             `json:"created_at"`
	UpdatedAt                         time.Time                             `json:"updated_at"`
}

func (n *NodeWithHost) Fill(_node *schema.Node) {
	n.ID = _node.ID
	n.HostID = _node.HostID
	n.Host = NewApiHostFromSchemaHost(_node.Host)
	n.NetworkID = _node.NetworkID
	n.Address = _node.Address
	n.Address6 = _node.Address6
	n.Connected = _node.Connected
	n.Action = _node.Action
	n.Status = _node.Status
	n.PendingDelete = _node.PendingDelete
	n.AutoAssignGateway = _node.AutoAssignGateway
	n.IsGateway = _node.IsGateway
	n.IsAutoRelay = _node.IsAutoRelay == "yes"
	n.IsInternetGateway = _node.IsInternetGateway
	n.RelayedClients = _node.RelayedClients
	n.RelayedIGWClients = _node.RelayedIGWClients
	n.RelayedByNodeID = _node.RelayedByNodeID
	n.IsIGWClient = _node.IsIGWClient
	n.AutoRelayedPeers = _node.AutoRelayedPeers
	n.Tags = _node.Tags
	n.PostureCheckSeverity = _node.PostureCheckSeverity
	n.PostureCheckLastEvaluationCycleID = _node.PostureCheckLastEvaluationCycleID
	n.Metadata = _node.Metadata
	n.LastCheckIn = _node.LastCheckIn
	n.ExpirationDateTime = _node.ExpirationDateTime
	n.CreatedAt = _node.CreatedAt
	n.UpdatedAt = _node.UpdatedAt
}
