package models

import (
	"net"
	"net/netip"

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
	Public     string
	Symmetric  string
	Asymmetric string
	Double     string
}{
	Public:     "public",
	Symmetric:  "symmetric",
	Asymmetric: "asymmetric",
	Double:     "double",
}

// WIREGUARD_INTERFACE name of wireguard interface
const WIREGUARD_INTERFACE = "netmaker"

// Host - represents a host on the network
type Host struct {
	ID               uuid.UUID        `json:"id" yaml:"id"`
	Verbosity        int              `json:"verbosity" yaml:"verbosity"`
	FirewallInUse    string           `json:"firewallinuse" yaml:"firewallinuse"`
	Version          string           `json:"version" yaml:"version"`
	IPForwarding     bool             `json:"ipforwarding" yaml:"ipforwarding"`
	DaemonInstalled  bool             `json:"daemoninstalled" yaml:"daemoninstalled"`
	HostPass         string           `json:"hostpass" yaml:"hostpass"`
	Name             string           `json:"name" yaml:"name"`
	OS               string           `json:"os" yaml:"os"`
	Interface        string           `json:"interface" yaml:"interface"`
	Debug            bool             `json:"debug" yaml:"debug"`
	ListenPort       int              `json:"listenport" yaml:"listenport"`
	PublicListenPort int              `json:"public_listen_port" yaml:"public_listen_port"`
	ProxyListenPort  int              `json:"proxy_listen_port" yaml:"proxy_listen_port"`
	MTU              int              `json:"mtu" yaml:"mtu"`
	PublicKey        wgtypes.Key      `json:"publickey" yaml:"publickey"`
	MacAddress       net.HardwareAddr `json:"macaddress" yaml:"macaddress"`
	TrafficKeyPublic []byte           `json:"traffickeypublic" yaml:"traffickeypublic"`
	InternetGateway  net.UDPAddr      `json:"internetgateway" yaml:"internetgateway"`
	Nodes            []string         `json:"nodes" yaml:"nodes"`
	IsRelayed        bool             `json:"isrelayed" yaml:"isrelayed"`
	RelayedBy        string           `json:"relayed_by" yaml:"relayed_by"`
	IsRelay          bool             `json:"isrelay" yaml:"isrelay"`
	RelayedHosts     []string         `json:"relay_hosts" yaml:"relay_hosts"`
	Interfaces       []Iface          `json:"interfaces" yaml:"interfaces"`
	DefaultInterface string           `json:"defaultinterface" yaml:"defaultinterface"`
	EndpointIP       net.IP           `json:"endpointip" yaml:"endpointip"`
	ShouldUseTurn    bool             `json:"should_use_turn" yaml:"should_use_turn"`
	TurnRelayAddr    net.PacketConn   `json:"turn_relay_addr" yaml:"turn_relay_addr"`
	ProxyEnabled     bool             `json:"proxy_enabled" yaml:"proxy_enabled"`
	ProxyEnabledSet  bool             `json:"proxy_enabled_updated" yaml:"proxy_enabled_updated"`
	IsDocker         bool             `json:"isdocker" yaml:"isdocker"`
	IsK8S            bool             `json:"isk8s" yaml:"isk8s"`
	IsStatic         bool             `json:"isstatic" yaml:"isstatic"`
	IsDefault        bool             `json:"isdefault" yaml:"isdefault"`
	NatType          string           `json:"nat_type,omitempty" yaml:"nat_type,omitempty"`
	TurnEndpoint     *netip.AddrPort  `json:"turn_endpoint,omitempty" yaml:"turn_endpoint,omitempty"`
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
	// SignalHost - const for host signal action
	SignalHost = "SIGNAL_HOST"
	// UpdateHost - constant for host update action
	UpdateHost = "UPDATE_HOST"
	// DeleteHost - constant for host delete action
	DeleteHost = "DELETE_HOST"
	// JoinHostToNetwork - constant for host network join action
	JoinHostToNetwork = "JOIN_HOST_TO_NETWORK"
	// Acknowledgement - ACK response for hosts
	Acknowledgement = "ACK"
	// RequestAck - request an ACK
	RequestAck = "REQ_ACK"
	// CheckIn - update last check in times and public address and interfaces
	CheckIn = "CHECK_IN"
	// REGISTER_WITH_TURN - registers host with turn server if configured
	RegisterWithTurn = "REGISTER_WITH_TURN"
)

// HostUpdate - struct for host update
type HostUpdate struct {
	Action HostMqAction
	Host   Host
	Node   Node
	Signal Signal
}

// HostTurnRegister - struct for host turn registration
type HostTurnRegister struct {
	HostID       string `json:"host_id"`
	HostPassHash string `json:"host_pass_hash"`
}

// Signal - struct for signalling peer
type Signal struct {
	FromHostPubKey    string `json:"from_host_pubkey"`
	TurnRelayEndpoint string `json:"turn_relay_addr"`
	ToHostPubKey      string `json:"to_host_pubkey"`
	Reply             bool   `json:"reply"`
}
