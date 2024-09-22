package models

import (
	"net"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// OS_Types - list of OS types Netmaker cares about
var OS_Types = struct {
	Linux   string
	Windows string
	Mac     string
	FreeBSD string
	IoT     string
}{
	Linux:   "linux",
	Windows: "windows",
	Mac:     "darwin",
	FreeBSD: "freebsd",
	IoT:     "iot",
}

// NAT_Types - the type of NAT in which a HOST currently resides (simplified)
var NAT_Types = struct {
	Public    string
	BehindNAT string
}{
	Public:    "public",
	BehindNAT: "behind_nat",
}

// WIREGUARD_INTERFACE name of wireguard interface
const (
	WIREGUARD_INTERFACE        = "netmaker"
	DefaultPersistentKeepAlive = 20 * time.Second
)

// Host - represents a host on the network
type Host struct {
	ID                  uuid.UUID        `json:"id"                      yaml:"id"`
	Verbosity           int              `json:"verbosity"               yaml:"verbosity"`
	FirewallInUse       string           `json:"firewallinuse"           yaml:"firewallinuse"`
	Version             string           `json:"version"                 yaml:"version"`
	IPForwarding        bool             `json:"ipforwarding"            yaml:"ipforwarding"`
	DaemonInstalled     bool             `json:"daemoninstalled"         yaml:"daemoninstalled"`
	AutoUpdate          bool             `json:"autoupdate"              yaml:"autoupdate"`
	HostPass            string           `json:"hostpass"                yaml:"hostpass"`
	Name                string           `json:"name"                    yaml:"name"`
	OS                  string           `json:"os"                      yaml:"os"`
	Interface           string           `json:"interface"               yaml:"interface"`
	Debug               bool             `json:"debug"                   yaml:"debug"`
	ListenPort          int              `json:"listenport"              yaml:"listenport"`
	WgPublicListenPort  int              `json:"wg_public_listen_port"   yaml:"wg_public_listen_port"`
	MTU                 int              `json:"mtu"                     yaml:"mtu"`
	PublicKey           wgtypes.Key      `json:"publickey"               yaml:"publickey"`
	MacAddress          net.HardwareAddr `json:"macaddress"              yaml:"macaddress"`
	TrafficKeyPublic    []byte           `json:"traffickeypublic"        yaml:"traffickeypublic"`
	Nodes               []string         `json:"nodes"                   yaml:"nodes"`
	Interfaces          []Iface          `json:"interfaces"              yaml:"interfaces"`
	DefaultInterface    string           `json:"defaultinterface"        yaml:"defaultinterface"`
	EndpointIP          net.IP           `json:"endpointip"              yaml:"endpointip"`
	EndpointIPv6        net.IP           `json:"endpointipv6"            yaml:"endpointipv6"`
	IsDocker            bool             `json:"isdocker"                yaml:"isdocker"`
	IsK8S               bool             `json:"isk8s"                   yaml:"isk8s"`
	IsStaticPort        bool             `json:"isstaticport"            yaml:"isstaticport"`
	IsStatic            bool             `json:"isstatic"        yaml:"isstatic"`
	IsDefault           bool             `json:"isdefault"               yaml:"isdefault"`
	NatType             string           `json:"nat_type,omitempty"      yaml:"nat_type,omitempty"`
	TurnEndpoint        *netip.AddrPort  `json:"turn_endpoint,omitempty" yaml:"turn_endpoint,omitempty"`
	PersistentKeepalive time.Duration    `json:"persistentkeepalive"     yaml:"persistentkeepalive"`
}

// FormatBool converts a boolean to a [yes|no] string
func FormatBool(b bool) string {
	s := "no"
	if b {
		s = "yes"
	}
	return s
}

// ParseBool parses a [yes|no] string to boolean value
func ParseBool(s string) bool {
	b := false
	if s == "yes" {
		b = true
	}
	return b
}

// HostMqAction - type for host update action
type HostMqAction string

const (
	// Upgrade - const to request host to update it's client
	Upgrade HostMqAction = "UPGRADE"
	// SignalHost - const for host signal action
	SignalHost HostMqAction = "SIGNAL_HOST"
	// UpdateHost - constant for host update action
	UpdateHost HostMqAction = "UPDATE_HOST"
	// DeleteHost - constant for host delete action
	DeleteHost HostMqAction = "DELETE_HOST"
	// JoinHostToNetwork - constant for host network join action
	JoinHostToNetwork HostMqAction = "JOIN_HOST_TO_NETWORK"
	// Acknowledgement - ACK response for hosts
	Acknowledgement HostMqAction = "ACK"
	// RequestAck - request an ACK
	RequestAck HostMqAction = "REQ_ACK"
	// CheckIn - update last check in times and public address and interfaces
	CheckIn HostMqAction = "CHECK_IN"
	// UpdateKeys - update wireguard private/public keys
	UpdateKeys HostMqAction = "UPDATE_KEYS"
	// RequestPull - request a pull from a host
	RequestPull HostMqAction = "REQ_PULL"
	// SignalPull - request a pull from a host without restart
	SignalPull HostMqAction = "SIGNAL_PULL"
	// UpdateMetrics - updates metrics data
	UpdateMetrics HostMqAction = "UPDATE_METRICS"
)

// SignalAction - turn peer signal action
type SignalAction string

const (
	// ConnNegotiation - action to negotiate connection between peers
	ConnNegotiation SignalAction = "CONNECTION_NEGOTIATION"
	// RelayME - action to relay the peer
	RelayME SignalAction = "RELAY_ME"
)

// HostUpdate - struct for host update
type HostUpdate struct {
	Action     HostMqAction
	Host       Host
	Node       Node
	Signal     Signal
	NewMetrics Metrics
}

// HostTurnRegister - struct for host turn registration
type HostTurnRegister struct {
	HostID       string `json:"host_id"`
	HostPassHash string `json:"host_pass_hash"`
}

// Signal - struct for signalling peer
type Signal struct {
	Server         string       `json:"server"`
	FromHostPubKey string       `json:"from_host_pubkey"`
	ToHostPubKey   string       `json:"to_host_pubkey"`
	FromHostID     string       `json:"from_host_id"`
	ToHostID       string       `json:"to_host_id"`
	FromNodeID     string       `json:"from_node_id"`
	ToNodeID       string       `json:"to_node_id"`
	Reply          bool         `json:"reply"`
	Action         SignalAction `json:"action"`
	IsPro          bool         `json:"is_pro"`
	TimeStamp      int64        `json:"timestamp"`
}

// RegisterMsg - login message struct for hosts to join via SSO login
type RegisterMsg struct {
	RegisterHost Host   `json:"host"`
	Network      string `json:"network,omitempty"`
	User         string `json:"user,omitempty"`
	Password     string `json:"password,omitempty"`
	JoinAll      bool   `json:"join_all,omitempty"`
	Relay        string `json:"relay,omitempty"`
}
