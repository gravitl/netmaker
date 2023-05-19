package models

import (
	"bytes"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	// NODE_SERVER_NAME - the default server name
	NODE_SERVER_NAME = "netmaker"
	// TEN_YEARS_IN_SECONDS - ten years in seconds
	TEN_YEARS_IN_SECONDS = 315670000000000000
	// MAX_NAME_LENGTH - max name length of node
	MAX_NAME_LENGTH = 62
	// == ACTIONS == (can only be set by server)
	// NODE_DELETE - delete node action
	NODE_DELETE = "delete"
	// NODE_IS_PENDING - node pending status
	NODE_IS_PENDING = "pending"
	// NODE_NOOP - node no op action
	NODE_NOOP = "noop"
	// NODE_FORCE_UPDATE - indicates a node should pull all changes
	NODE_FORCE_UPDATE = "force"
	// FIREWALL_IPTABLES - indicates that iptables is the firewall in use
	FIREWALL_IPTABLES = "iptables"
	// FIREWALL_NFTABLES - indicates nftables is in use (Linux only)
	FIREWALL_NFTABLES = "nftables"
	// FIREWALL_NONE - indicates that no supported firewall in use
	FIREWALL_NONE = "none"
)

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()))

// NodeCheckin - struct for node checkins with server
type NodeCheckin struct {
	Version   string
	Connected bool
	Ifaces    []Iface
}

// Iface struct for local interfaces of a node
type Iface struct {
	Name          string    `json:"name"`
	Address       net.IPNet `json:"address"`
	AddressString string    `json:"addressString"`
}

// CommonNode - represents a commonn node data elements shared by netmaker and netclient
type CommonNode struct {
	ID                  uuid.UUID     `json:"id" yaml:"id"`
	HostID              uuid.UUID     `json:"hostid" yaml:"hostid"`
	Network             string        `json:"network" yaml:"network"`
	NetworkRange        net.IPNet     `json:"networkrange" yaml:"networkrange"`
	NetworkRange6       net.IPNet     `json:"networkrange6" yaml:"networkrange6"`
	InternetGateway     *net.UDPAddr  `json:"internetgateway" yaml:"internetgateway"`
	Server              string        `json:"server" yaml:"server"`
	Connected           bool          `json:"connected" yaml:"connected"`
	Address             net.IPNet     `json:"address" yaml:"address"`
	Address6            net.IPNet     `json:"address6" yaml:"address6"`
	Action              string        `json:"action" yaml:"action"`
	LocalAddress        net.IPNet     `json:"localaddress" yaml:"localaddress"`
	IsEgressGateway     bool          `json:"isegressgateway" yaml:"isegressgateway"`
	EgressGatewayRanges []string      `json:"egressgatewayranges" bson:"egressgatewayranges" yaml:"egressgatewayranges"`
	IsIngressGateway    bool          `json:"isingressgateway" yaml:"isingressgateway"`
	IngressDNS          string        `json:"ingressdns" yaml:"ingressdns"`
	DNSOn               bool          `json:"dnson" yaml:"dnson"`
	PersistentKeepalive time.Duration `json:"persistentkeepalive" yaml:"persistentkeepalive"`
}

// Node - a model of a network node
type Node struct {
	CommonNode
	PendingDelete           bool                 `json:"pendingdelete" bson:"pendingdelete" yaml:"pendingdelete"`
	LastModified            time.Time            `json:"lastmodified" bson:"lastmodified" yaml:"lastmodified"`
	LastCheckIn             time.Time            `json:"lastcheckin" bson:"lastcheckin" yaml:"lastcheckin"`
	LastPeerUpdate          time.Time            `json:"lastpeerupdate" bson:"lastpeerupdate" yaml:"lastpeerupdate"`
	ExpirationDateTime      time.Time            `json:"expdatetime" bson:"expdatetime" yaml:"expdatetime"`
	EgressGatewayNatEnabled bool                 `json:"egressgatewaynatenabled" bson:"egressgatewaynatenabled" yaml:"egressgatewaynatenabled"`
	EgressGatewayRequest    EgressGatewayRequest `json:"egressgatewayrequest" bson:"egressgatewayrequest" yaml:"egressgatewayrequest"`
	IngressGatewayRange     string               `json:"ingressgatewayrange" bson:"ingressgatewayrange" yaml:"ingressgatewayrange"`
	IngressGatewayRange6    string               `json:"ingressgatewayrange6" bson:"ingressgatewayrange6" yaml:"ingressgatewayrange6"`
	IsRelayed               bool                 `json:"isrelayed" bson:"isrelayed" yaml:"isrelayed"`
	IsRelay                 bool                 `json:"isrelay" bson:"isrelay" yaml:"isrelay"`
	RelayAddrs              []string             `json:"relayaddrs" bson:"relayaddrs" yaml:"relayaddrs"`
	// == PRO ==
	DefaultACL   string    `json:"defaultacl,omitempty" bson:"defaultacl,omitempty" yaml:"defaultacl,omitempty" validate:"checkyesornoorunset"`
	OwnerID      string    `json:"ownerid,omitempty" bson:"ownerid,omitempty" yaml:"ownerid,omitempty"`
	FailoverNode uuid.UUID `json:"failovernode" bson:"failovernode" yaml:"failovernode"`
	Failover     bool      `json:"failover" bson:"failover" yaml:"failover"`
}

// LegacyNode - legacy struct for node model
type LegacyNode struct {
	ID                      string               `json:"id,omitempty" bson:"id,omitempty" yaml:"id,omitempty" validate:"required,min=5,id_unique"`
	HostID                  string               `json:"hostid,omitempty" bson:"id,omitempty" yaml:"hostid,omitempty" validate:"required,min=5,id_unique"`
	Address                 string               `json:"address" bson:"address" yaml:"address" validate:"omitempty,ipv4"`
	Address6                string               `json:"address6" bson:"address6" yaml:"address6" validate:"omitempty,ipv6"`
	LocalAddress            string               `json:"localaddress" bson:"localaddress" yaml:"localaddress" validate:"omitempty"`
	Interfaces              []Iface              `json:"interfaces" yaml:"interfaces"`
	Name                    string               `json:"name" bson:"name" yaml:"name" validate:"omitempty,max=62,in_charset"`
	NetworkSettings         Network              `json:"networksettings" bson:"networksettings" yaml:"networksettings" validate:"-"`
	ListenPort              int32                `json:"listenport" bson:"listenport" yaml:"listenport" validate:"omitempty,numeric,min=1024,max=65535"`
	LocalListenPort         int32                `json:"locallistenport" bson:"locallistenport" yaml:"locallistenport" validate:"numeric,min=0,max=65535"`
	ProxyListenPort         int32                `json:"proxy_listen_port" bson:"proxy_listen_port" yaml:"proxy_listen_port" validate:"numeric,min=0,max=65535"`
	PublicKey               string               `json:"publickey" bson:"publickey" yaml:"publickey" validate:"required,base64"`
	Endpoint                string               `json:"endpoint" bson:"endpoint" yaml:"endpoint" validate:"required,ip"`
	AllowedIPs              []string             `json:"allowedips" bson:"allowedips" yaml:"allowedips"`
	PersistentKeepalive     int32                `json:"persistentkeepalive" bson:"persistentkeepalive" yaml:"persistentkeepalive" validate:"omitempty,numeric,max=1000"`
	IsHub                   string               `json:"ishub" bson:"ishub" yaml:"ishub" validate:"checkyesorno"`
	AccessKey               string               `json:"accesskey" bson:"accesskey" yaml:"accesskey"`
	Interface               string               `json:"interface" bson:"interface" yaml:"interface"`
	LastModified            int64                `json:"lastmodified" bson:"lastmodified" yaml:"lastmodified"`
	ExpirationDateTime      int64                `json:"expdatetime" bson:"expdatetime" yaml:"expdatetime"`
	LastPeerUpdate          int64                `json:"lastpeerupdate" bson:"lastpeerupdate" yaml:"lastpeerupdate"`
	LastCheckIn             int64                `json:"lastcheckin" bson:"lastcheckin" yaml:"lastcheckin"`
	MacAddress              string               `json:"macaddress" bson:"macaddress" yaml:"macaddress"`
	Password                string               `json:"password" bson:"password" yaml:"password" validate:"required,min=6"`
	Network                 string               `json:"network" bson:"network" yaml:"network" validate:"network_exists"`
	IsRelayed               string               `json:"isrelayed" bson:"isrelayed" yaml:"isrelayed"`
	IsPending               string               `json:"ispending" bson:"ispending" yaml:"ispending"`
	IsRelay                 string               `json:"isrelay" bson:"isrelay" yaml:"isrelay" validate:"checkyesorno"`
	IsDocker                string               `json:"isdocker" bson:"isdocker" yaml:"isdocker" validate:"checkyesorno"`
	IsK8S                   string               `json:"isk8s" bson:"isk8s" yaml:"isk8s" validate:"checkyesorno"`
	IsEgressGateway         string               `json:"isegressgateway" bson:"isegressgateway" yaml:"isegressgateway" validate:"checkyesorno"`
	IsIngressGateway        string               `json:"isingressgateway" bson:"isingressgateway" yaml:"isingressgateway" validate:"checkyesorno"`
	EgressGatewayRanges     []string             `json:"egressgatewayranges" bson:"egressgatewayranges" yaml:"egressgatewayranges"`
	EgressGatewayNatEnabled string               `json:"egressgatewaynatenabled" bson:"egressgatewaynatenabled" yaml:"egressgatewaynatenabled"`
	EgressGatewayRequest    EgressGatewayRequest `json:"egressgatewayrequest" bson:"egressgatewayrequest" yaml:"egressgatewayrequest"`
	RelayAddrs              []string             `json:"relayaddrs" bson:"relayaddrs" yaml:"relayaddrs"`
	FailoverNode            string               `json:"failovernode" bson:"failovernode" yaml:"failovernode"`
	IngressGatewayRange     string               `json:"ingressgatewayrange" bson:"ingressgatewayrange" yaml:"ingressgatewayrange"`
	IngressGatewayRange6    string               `json:"ingressgatewayrange6" bson:"ingressgatewayrange6" yaml:"ingressgatewayrange6"`
	// IsStatic - refers to if the Endpoint is set manually or dynamically
	IsStatic        string      `json:"isstatic" bson:"isstatic" yaml:"isstatic" validate:"checkyesorno"`
	UDPHolePunch    string      `json:"udpholepunch" bson:"udpholepunch" yaml:"udpholepunch" validate:"checkyesorno"`
	DNSOn           string      `json:"dnson" bson:"dnson" yaml:"dnson" validate:"checkyesorno"`
	IsServer        string      `json:"isserver" bson:"isserver" yaml:"isserver" validate:"checkyesorno"`
	Action          string      `json:"action" bson:"action" yaml:"action"`
	IPForwarding    string      `json:"ipforwarding" bson:"ipforwarding" yaml:"ipforwarding" validate:"checkyesorno"`
	OS              string      `json:"os" bson:"os" yaml:"os"`
	MTU             int32       `json:"mtu" bson:"mtu" yaml:"mtu"`
	Version         string      `json:"version" bson:"version" yaml:"version"`
	Server          string      `json:"server" bson:"server" yaml:"server"`
	TrafficKeys     TrafficKeys `json:"traffickeys" bson:"traffickeys" yaml:"traffickeys"`
	FirewallInUse   string      `json:"firewallinuse" bson:"firewallinuse" yaml:"firewallinuse"`
	InternetGateway string      `json:"internetgateway" bson:"internetgateway" yaml:"internetgateway"`
	Connected       string      `json:"connected" bson:"connected" yaml:"connected" validate:"checkyesorno"`
	PendingDelete   bool        `json:"pendingdelete" bson:"pendingdelete" yaml:"pendingdelete"`
	Proxy           bool        `json:"proxy" bson:"proxy" yaml:"proxy"`
	// == PRO ==
	DefaultACL string `json:"defaultacl,omitempty" bson:"defaultacl,omitempty" yaml:"defaultacl,omitempty" validate:"checkyesornoorunset"`
	OwnerID    string `json:"ownerid,omitempty" bson:"ownerid,omitempty" yaml:"ownerid,omitempty"`
	Failover   string `json:"failover" bson:"failover" yaml:"failover" validate:"checkyesorno"`
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
func (node *Node) PrimaryAddress() string {
	if node.Address.IP != nil {
		return node.Address.IP.String()
	}
	return node.Address6.IP.String()
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

// Node.SetDefaultACL
func (node *LegacyNode) SetDefaultACL() {
	if node.DefaultACL == "" {
		node.DefaultACL = "yes"
	}
}

// Node.SetDefaultMTU - sets default MTU of a node
func (node *LegacyNode) SetDefaultMTU() {
	if node.MTU == 0 {
		node.MTU = 1280
	}
}

// Node.SetDefaultNFTablesPresent - sets default for nftables check
func (node *LegacyNode) SetDefaultNFTablesPresent() {
	if node.FirewallInUse == "" {
		node.FirewallInUse = FIREWALL_IPTABLES // default to iptables
	}
}

// Node.SetDefaultIsRelayed - set default is relayed
func (node *LegacyNode) SetDefaultIsRelayed() {
	if node.IsRelayed == "" {
		node.IsRelayed = "no"
	}
}

// Node.SetDefaultIsRelayed - set default is relayed
func (node *LegacyNode) SetDefaultIsHub() {
	if node.IsHub == "" {
		node.IsHub = "no"
	}
}

// Node.SetDefaultIsRelay - set default isrelay
func (node *LegacyNode) SetDefaultIsRelay() {
	if node.IsRelay == "" {
		node.IsRelay = "no"
	}
}

// Node.SetDefaultIsDocker - set default isdocker
func (node *LegacyNode) SetDefaultIsDocker() {
	if node.IsDocker == "" {
		node.IsDocker = "no"
	}
}

// Node.SetDefaultIsK8S - set default isk8s
func (node *LegacyNode) SetDefaultIsK8S() {
	if node.IsK8S == "" {
		node.IsK8S = "no"
	}
}

// Node.SetDefaultEgressGateway - sets default egress gateway status
func (node *LegacyNode) SetDefaultEgressGateway() {
	if node.IsEgressGateway == "" {
		node.IsEgressGateway = "no"
	}
}

// Node.SetDefaultIngressGateway - sets default ingress gateway status
func (node *LegacyNode) SetDefaultIngressGateway() {
	if node.IsIngressGateway == "" {
		node.IsIngressGateway = "no"
	}
}

// Node.SetDefaultAction - sets default action status
func (node *LegacyNode) SetDefaultAction() {
	if node.Action == "" {
		node.Action = NODE_NOOP
	}
}

// Node.SetRoamingDefault - sets default roaming status
//func (node *Node) SetRoamingDefault() {
//	if node.Roaming == "" {
//		node.Roaming = "yes"
//	}
//}

// Node.SetIPForwardingDefault - set ip forwarding default
func (node *LegacyNode) SetIPForwardingDefault() {
	if node.IPForwarding == "" {
		node.IPForwarding = "yes"
	}
}

// Node.SetDNSOnDefault - sets dns on default
func (node *LegacyNode) SetDNSOnDefault() {
	if node.DNSOn == "" {
		node.DNSOn = "yes"
	}
}

// Node.SetIsServerDefault - sets node isserver default
func (node *LegacyNode) SetIsServerDefault() {
	if node.IsServer != "yes" {
		node.IsServer = "no"
	}
}

// Node.SetIsStaticDefault - set is static default
func (node *LegacyNode) SetIsStaticDefault() {
	if node.IsServer == "yes" {
		node.IsStatic = "yes"
	} else if node.IsStatic != "yes" {
		node.IsStatic = "no"
	}
}

// Node.SetLastModified - set last modified initial time
func (node *Node) SetLastModified() {
	node.LastModified = time.Now()
}

// Node.SetLastCheckIn - set checkin time of node
func (node *Node) SetLastCheckIn() {
	node.LastCheckIn = time.Now()
}

// Node.SetLastPeerUpdate - sets last peer update time
func (node *Node) SetLastPeerUpdate() {
	node.LastPeerUpdate = time.Now()
}

// Node.SetExpirationDateTime - sets node expiry time
func (node *Node) SetExpirationDateTime() {
	node.ExpirationDateTime = time.Now().Add(TEN_YEARS_IN_SECONDS)
}

// Node.SetDefaultName - sets a random name to node
func (node *LegacyNode) SetDefaultName() {
	if node.Name == "" {
		node.Name = GenerateNodeName()
	}
}

// Node.SetDefaultFailover - sets default value of failover status to no if not set
func (node *LegacyNode) SetDefaultFailover() {
	if node.Failover == "" {
		node.Failover = "no"
	}
}

// Node.Fill - fills other node data into calling node data if not set on calling node
func (newNode *Node) Fill(currentNode *Node) { // TODO add new field for nftables present
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
	if newNode.PersistentKeepalive < 0 {
		newNode.PersistentKeepalive = currentNode.PersistentKeepalive
	}
	if newNode.LastModified != currentNode.LastModified {
		newNode.LastModified = currentNode.LastModified
	}
	if newNode.ExpirationDateTime.IsZero() {
		newNode.ExpirationDateTime = currentNode.ExpirationDateTime
	}
	if newNode.LastPeerUpdate.IsZero() {
		newNode.LastPeerUpdate = currentNode.LastPeerUpdate
	}
	if newNode.LastCheckIn.IsZero() {
		newNode.LastCheckIn = currentNode.LastCheckIn
	}
	if newNode.Network == "" {
		newNode.Network = currentNode.Network
	}
	if newNode.IsEgressGateway != currentNode.IsEgressGateway {
		newNode.IsEgressGateway = currentNode.IsEgressGateway
	}
	if newNode.IsIngressGateway != currentNode.IsIngressGateway {
		newNode.IsIngressGateway = currentNode.IsIngressGateway
	}
	if newNode.EgressGatewayRanges == nil {
		newNode.EgressGatewayRanges = currentNode.EgressGatewayRanges
	}
	if newNode.IngressGatewayRange == "" {
		newNode.IngressGatewayRange = currentNode.IngressGatewayRange
	}
	if newNode.IngressGatewayRange6 == "" {
		newNode.IngressGatewayRange6 = currentNode.IngressGatewayRange6
	}
	if newNode.DNSOn != currentNode.DNSOn {
		newNode.DNSOn = currentNode.DNSOn
	}
	if newNode.Action == "" {
		newNode.Action = currentNode.Action
	}
	if newNode.RelayAddrs == nil {
		newNode.RelayAddrs = currentNode.RelayAddrs
	}
	if newNode.IsRelay != currentNode.IsRelay {
		newNode.IsRelay = currentNode.IsRelay
	}
	if newNode.IsRelayed == currentNode.IsRelayed {
		newNode.IsRelayed = currentNode.IsRelayed
	}
	if newNode.Server == "" {
		newNode.Server = currentNode.Server
	}
	if newNode.DefaultACL == "" {
		newNode.DefaultACL = currentNode.DefaultACL
	}
	if newNode.Failover != currentNode.Failover {
		newNode.Failover = currentNode.Failover
	}
}

// StringWithCharset - returns random string inside defined charset
func StringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

// IsIpv4Net - check for valid IPv4 address
// Note: We dont handle IPv6 AT ALL!!!!! This definitely is needed at some point
// But for iteration 1, lets just stick to IPv4. Keep it simple stupid.
func IsIpv4Net(host string) bool {
	return net.ParseIP(host) != nil
}

// Node.NameInNodeCharset - returns if name is in charset below or not
func (node *LegacyNode) NameInNodeCharSet() bool {

	charset := "abcdefghijklmnopqrstuvwxyz1234567890-"

	for _, char := range node.Name {
		if !strings.Contains(charset, strings.ToLower(string(char))) {
			return false
		}
	}
	return true
}

// == PRO ==

// Node.DoesACLAllow - checks if default ACL on node is "yes"
func (node *Node) DoesACLAllow() bool {
	return node.DefaultACL == "yes"
}

// Node.DoesACLDeny - checks if default ACL on node is "no"
func (node *Node) DoesACLDeny() bool {
	return node.DefaultACL == "no"
}

func (ln *LegacyNode) ConvertToNewNode() (*Host, *Node) {
	var node Node
	//host:= logic.GetHost(node.HostID)
	var host Host
	if host.ID.String() == "" {
		host.ID = uuid.New()
		host.FirewallInUse = ln.FirewallInUse
		host.Version = ln.Version
		host.IPForwarding = parseBool(ln.IPForwarding)
		host.HostPass = ln.Password
		host.Name = ln.Name
		host.ListenPort = int(ln.ListenPort)
		host.ProxyListenPort = int(ln.ProxyListenPort)
		host.MTU = int(ln.MTU)
		host.PublicKey, _ = wgtypes.ParseKey(ln.PublicKey)
		host.MacAddress, _ = net.ParseMAC(ln.MacAddress)
		host.TrafficKeyPublic = ln.TrafficKeys.Mine
		gateway, err := net.ResolveUDPAddr("udp", ln.InternetGateway)
		if err == nil {
			host.InternetGateway = *gateway
		}
		id, _ := uuid.Parse(ln.ID)
		host.Nodes = append(host.Nodes, id.String())
		host.Interfaces = ln.Interfaces
		host.EndpointIP = net.ParseIP(ln.Endpoint)
		// host.ProxyEnabled = ln.Proxy // this will always be false..
	}
	id, _ := uuid.Parse(ln.ID)
	node.ID = id
	node.Network = ln.Network
	if _, cidr, err := net.ParseCIDR(ln.NetworkSettings.AddressRange); err == nil {
		node.NetworkRange = *cidr
	}
	if _, cidr, err := net.ParseCIDR(ln.NetworkSettings.AddressRange6); err == nil {
		node.NetworkRange6 = *cidr
	}
	node.Server = ln.Server
	node.Connected = parseBool(ln.Connected)
	if ln.Address != "" {
		node.Address = net.IPNet{
			IP:   net.ParseIP(ln.Address),
			Mask: net.CIDRMask(32, 32),
		}
	}
	if ln.Address6 != "" {
		node.Address = net.IPNet{
			IP:   net.ParseIP(ln.Address6),
			Mask: net.CIDRMask(128, 128),
		}
	}
	node.Action = ln.Action
	node.IsEgressGateway = parseBool(ln.IsEgressGateway)
	node.IsIngressGateway = parseBool(ln.IsIngressGateway)
	node.DNSOn = parseBool(ln.DNSOn)

	return &host, &node
}

// Node.Legacy converts node to legacy format
func (n *Node) Legacy(h *Host, s *ServerConfig, net *Network) *LegacyNode {
	l := LegacyNode{}
	l.ID = n.ID.String()
	l.HostID = h.ID.String()
	l.Address = n.Address.String()
	l.Address6 = n.Address6.String()
	l.Interfaces = h.Interfaces
	l.Name = h.Name
	l.NetworkSettings = *net
	l.ListenPort = int32(h.ListenPort)
	l.ProxyListenPort = int32(h.ProxyListenPort)
	l.PublicKey = h.PublicKey.String()
	l.Endpoint = h.EndpointIP.String()
	//l.AllowedIPs =
	l.AccessKey = ""
	l.Interface = WIREGUARD_INTERFACE
	//l.LastModified =
	//l.ExpirationDateTime
	//l.LastPeerUpdate
	//l.LastCheckIn
	l.MacAddress = h.MacAddress.String()
	l.Password = h.HostPass
	l.Network = n.Network
	//l.IsRelayed = formatBool(n.Is)
	//l.IsRelay = formatBool(n.IsRelay)
	//l.IsDocker = formatBool(n.IsDocker)
	//l.IsK8S = formatBool(n.IsK8S)
	l.IsEgressGateway = formatBool(n.IsEgressGateway)
	l.IsIngressGateway = formatBool(n.IsIngressGateway)
	//l.EgressGatewayRanges = n.EgressGatewayRanges
	//l.EgressGatewayNatEnabled = n.EgressGatewayNatEnabled
	//l.RelayAddrs = n.RelayAddrs
	//l.FailoverNode = n.FailoverNode
	//l.IngressGatewayRange = n.IngressGatewayRange
	//l.IngressGatewayRange6 = n.IngressGatewayRange6
	l.IsStatic = formatBool(h.IsStatic)
	l.UDPHolePunch = formatBool(true)
	l.DNSOn = formatBool(n.DNSOn)
	l.Action = n.Action
	l.IPForwarding = formatBool(h.IPForwarding)
	l.OS = h.OS
	l.MTU = int32(h.MTU)
	l.Version = h.Version
	l.Server = n.Server
	l.TrafficKeys.Mine = h.TrafficKeyPublic
	l.TrafficKeys.Server = s.TrafficKey
	l.FirewallInUse = h.FirewallInUse
	l.InternetGateway = h.InternetGateway.String()
	l.Connected = formatBool(n.Connected)
	//l.PendingDelete = formatBool(n.PendingDelete)
	l.Proxy = h.ProxyEnabled
	l.DefaultACL = n.DefaultACL
	l.OwnerID = n.OwnerID
	//l.Failover = n.Failover
	return &l
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

func parseBool(s string) bool {
	b := false
	if s == "yes" {
		b = true
	}
	return b
}

func formatBool(b bool) string {
	s := "no"
	if b {
		s = "yes"
	}
	return s
}
