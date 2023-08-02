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
	Public    string
	BehindNAT string
}{
	Public:    "public",
	BehindNAT: "behind_nat",
}

// WIREGUARD_INTERFACE name of wireguard interface
const WIREGUARD_INTERFACE = "netmaker"

// Host - represents a host on the network
type Host struct {
	ID                 uuid.UUID        `json:"id" yaml:"id"`
	Verbosity          int              `json:"verbosity" yaml:"verbosity"`
	FirewallInUse      string           `json:"firewallinuse" yaml:"firewallinuse"`
	Version            string           `json:"version" yaml:"version"`
	IPForwarding       bool             `json:"ipforwarding" yaml:"ipforwarding"`
	DaemonInstalled    bool             `json:"daemoninstalled" yaml:"daemoninstalled"`
	AutoUpdate         bool             `json:"autoupdate" yaml:"autoupdate"`
	HostPass           string           `json:"hostpass" yaml:"hostpass"`
	Name               string           `json:"name" yaml:"name"`
	OS                 string           `json:"os" yaml:"os"`
	Interface          string           `json:"interface" yaml:"interface"`
	Debug              bool             `json:"debug" yaml:"debug"`
	ListenPort         int              `json:"listenport" yaml:"listenport"`
	WgPublicListenPort int              `json:"wg_public_listen_port" yaml:"wg_public_listen_port"`
	MTU                int              `json:"mtu" yaml:"mtu"`
	PublicKey          wgtypes.Key      `json:"publickey" yaml:"publickey"`
	MacAddress         net.HardwareAddr `json:"macaddress" yaml:"macaddress"`
	TrafficKeyPublic   []byte           `json:"traffickeypublic" yaml:"traffickeypublic"`
	Nodes              []string         `json:"nodes" yaml:"nodes"`
	Interfaces         []Iface          `json:"interfaces" yaml:"interfaces"`
	DefaultInterface   string           `json:"defaultinterface" yaml:"defaultinterface"`
	EndpointIP         net.IP           `json:"endpointip" yaml:"endpointip"`
	IsDocker           bool             `json:"isdocker" yaml:"isdocker"`
	IsK8S              bool             `json:"isk8s" yaml:"isk8s"`
	IsStatic           bool             `json:"isstatic" yaml:"isstatic"`
	IsDefault          bool             `json:"isdefault" yaml:"isdefault"`
	NatType            string           `json:"nat_type,omitempty" yaml:"nat_type,omitempty"`
	TurnEndpoint       *netip.AddrPort  `json:"turn_endpoint,omitempty" yaml:"turn_endpoint,omitempty"`
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
	// UpdateKeys - update wireguard private/public keys
	UpdateKeys = "UPDATE_KEYS"
)

// SignalAction - turn peer signal action
type SignalAction string

const (
	// Disconnect - action to stop using turn connection
	Disconnect SignalAction = "DISCONNECT"
	// ConnNegotiation - action to negotiate connection between peers
	ConnNegotiation SignalAction = "CONNECTION_NEGOTIATION"
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
	Server            string       `json:"server"`
	FromHostPubKey    string       `json:"from_host_pubkey"`
	TurnRelayEndpoint string       `json:"turn_relay_addr"`
	ToHostPubKey      string       `json:"to_host_pubkey"`
	Reply             bool         `json:"reply"`
	Action            SignalAction `json:"action"`
	TimeStamp         int64        `json:"timestamp"`
}

// RegisterMsg - login message struct for hosts to join via SSO login
type RegisterMsg struct {
	RegisterHost Host   `json:"host"`
	Network      string `json:"network,omitempty"`
	User         string `json:"user,omitempty"`
	Password     string `json:"password,omitempty"`
	JoinAll      bool   `json:"join_all,omitempty"`
}
