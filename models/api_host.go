package models

import (
	"net"
	"strings"
	"time"
)

// ApiHost - the host struct for API usage
type ApiHost struct {
	ID                  string   `json:"id"`
	Verbosity           int      `json:"verbosity"`
	FirewallInUse       string   `json:"firewallinuse"`
	Version             string   `json:"version"`
	Name                string   `json:"name"`
	OS                  string   `json:"os"`
	Debug               bool     `json:"debug"`
	IsStatic            bool     `json:"isstatic"`
	ListenPort          int      `json:"listenport"`
	WgPublicListenPort  int      `json:"wg_public_listen_port" yaml:"wg_public_listen_port"`
	MTU                 int      `json:"mtu" yaml:"mtu"`
	Interfaces          []Iface  `json:"interfaces" yaml:"interfaces"`
	DefaultInterface    string   `json:"defaultinterface" yaml:"defautlinterface"`
	EndpointIP          string   `json:"endpointip" yaml:"endpointip"`
	PublicKey           string   `json:"publickey"`
	MacAddress          string   `json:"macaddress"`
	Nodes               []string `json:"nodes"`
	IsDefault           bool     `json:"isdefault" yaml:"isdefault"`
	IsRelayed           bool     `json:"isrelayed" bson:"isrelayed" yaml:"isrelayed"`
	RelayedBy           string   `json:"relayed_by" bson:"relayed_by" yaml:"relayed_by"`
	IsRelay             bool     `json:"isrelay" bson:"isrelay" yaml:"isrelay"`
	RelayedHosts        []string `json:"relay_hosts" bson:"relay_hosts" yaml:"relay_hosts"`
	NatType             string   `json:"nat_type" yaml:"nat_type"`
	PersistentKeepalive int32    `json:"persistentkeepalive" yaml:"persistentkeepalive"`
}

// Host.ConvertNMHostToAPI - converts a Netmaker host to an API editable host
func (h *Host) ConvertNMHostToAPI() *ApiHost {
	a := ApiHost{}
	a.Debug = h.Debug
	a.EndpointIP = h.EndpointIP.String()
	a.FirewallInUse = h.FirewallInUse
	a.ID = h.ID.String()
	a.Interfaces = h.Interfaces
	for i := range a.Interfaces {
		a.Interfaces[i].AddressString = a.Interfaces[i].Address.String()
	}
	a.DefaultInterface = h.DefaultInterface
	a.IsStatic = h.IsStatic
	a.ListenPort = h.ListenPort
	a.MTU = h.MTU
	a.MacAddress = h.MacAddress.String()
	a.Name = h.Name
	a.OS = h.OS
	a.Nodes = h.Nodes
	a.WgPublicListenPort = h.WgPublicListenPort
	a.PublicKey = h.PublicKey.String()
	a.Verbosity = h.Verbosity
	a.Version = h.Version
	a.IsDefault = h.IsDefault
	a.NatType = h.NatType
	a.PersistentKeepalive = int32(h.PersistentKeepalive)
	return &a
}

// APIHost.ConvertAPIHostToNMHost - convert's a given apihost struct to
// a Host struct
func (a *ApiHost) ConvertAPIHostToNMHost(currentHost *Host) *Host {
	h := Host{}
	h.ID = currentHost.ID
	h.HostPass = currentHost.HostPass
	h.DaemonInstalled = currentHost.DaemonInstalled
	if len(a.EndpointIP) == 0 || strings.Contains(a.EndpointIP, "nil") {
		h.EndpointIP = currentHost.EndpointIP
	} else {
		h.EndpointIP = net.ParseIP(a.EndpointIP)
	}
	h.Debug = a.Debug
	h.FirewallInUse = a.FirewallInUse
	h.IPForwarding = currentHost.IPForwarding
	h.Interface = currentHost.Interface
	h.Interfaces = currentHost.Interfaces
	h.DefaultInterface = currentHost.DefaultInterface
	h.IsDocker = currentHost.IsDocker
	h.IsK8S = currentHost.IsK8S
	h.IsStatic = a.IsStatic
	h.ListenPort = a.ListenPort
	h.MTU = a.MTU
	h.MacAddress = currentHost.MacAddress
	h.PublicKey = currentHost.PublicKey
	h.Name = a.Name
	h.Version = currentHost.Version
	h.Verbosity = a.Verbosity
	h.Nodes = currentHost.Nodes
	h.TrafficKeyPublic = currentHost.TrafficKeyPublic
	h.OS = currentHost.OS
	h.IsDefault = a.IsDefault
	h.NatType = currentHost.NatType
	h.TurnEndpoint = currentHost.TurnEndpoint
	if h.PersistentKeepalive == 0 {
		h.PersistentKeepalive = time.Duration(a.PersistentKeepalive)
	} else {
		h.PersistentKeepalive = currentHost.PersistentKeepalive
	}
	return &h
}
