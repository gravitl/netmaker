package models

import (
	"net"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type HostPeerInfo struct {
	NetworkPeerIDs map[NetworkID]PeerMap `json:"network_peers"`
}

// HostPeerUpdate - struct for host peer updates
type HostPeerUpdate struct {
	Host            Host                  `json:"host"`
	ChangeDefaultGw bool                  `json:"change_default_gw"`
	DefaultGwIp     net.IP                `json:"default_gw_ip"`
	IsInternetGw    bool                  `json:"is_inet_gw"`
	NodeAddrs       []net.IPNet           `json:"nodes_addrs"`
	Server          string                `json:"server"`
	ServerVersion   string                `json:"serverversion"`
	ServerAddrs     []ServerAddr          `json:"serveraddrs"`
	NodePeers       []wgtypes.PeerConfig  `json:"node_peers"`
	Peers           []wgtypes.PeerConfig  `json:"host_peers"`
	PeerIDs         PeerMap               `json:"peerids"`
	HostNetworkInfo HostInfoMap           `json:"host_network_info,omitempty"`
	EgressRoutes    []EgressNetworkRoutes `json:"egress_network_routes"`
	FwUpdate        FwUpdate              `json:"fw_update"`
	ReplacePeers    bool                  `json:"replace_peers"`
	NameServers     []string              `json:"name_servers"`
	ServerConfig
	OldPeerUpdateFields
}

type OldPeerUpdateFields struct {
	NodePeers         []wgtypes.PeerConfig `json:"peers" bson:"peers" yaml:"peers"`
	OldPeers          []wgtypes.PeerConfig `json:"Peers"`
	EndpointDetection bool                 `json:"endpoint_detection"`
}

type FwRule struct {
	SrcIP           net.IPNet `json:"src_ip"`
	DstIP           net.IPNet `json:"dst_ip"`
	AllowedProtocol Protocol  `json:"allowed_protocols"` // tcp, udp, etc.
	AllowedPorts    []string  `json:"allowed_ports"`
	Allow           bool      `json:"allow"`
}

// IngressInfo - struct for ingress info
type IngressInfo struct {
	IngressID     string      `json:"ingress_id"`
	Network       net.IPNet   `json:"network"`
	Network6      net.IPNet   `json:"network6"`
	StaticNodeIps []net.IP    `json:"static_node_ips"`
	Rules         []FwRule    `json:"rules"`
	AllowAll      bool        `json:"allow_all"`
	EgressRanges  []net.IPNet `json:"egress_ranges"`
	EgressRanges6 []net.IPNet `json:"egress_ranges6"`
}

// EgressInfo - struct for egress info
type EgressInfo struct {
	EgressID      string               `json:"egress_id" yaml:"egress_id"`
	Network       net.IPNet            `json:"network" yaml:"network"`
	EgressGwAddr  net.IPNet            `json:"egress_gw_addr" yaml:"egress_gw_addr"`
	Network6      net.IPNet            `json:"network6" yaml:"network6"`
	EgressGwAddr6 net.IPNet            `json:"egress_gw_addr6" yaml:"egress_gw_addr6"`
	EgressGWCfg   EgressGatewayRequest `json:"egress_gateway_cfg" yaml:"egress_gateway_cfg"`
}

// EgressNetworkRoutes - struct for egress network routes for adding routes to peer's interface
type EgressNetworkRoutes struct {
	EgressGwAddr  net.IPNet `json:"egress_gw_addr" yaml:"egress_gw_addr"`
	EgressGwAddr6 net.IPNet `json:"egress_gw_addr6" yaml:"egress_gw_addr6"`
	NodeAddr      net.IPNet `json:"node_addr"`
	NodeAddr6     net.IPNet `json:"node_addr6"`
	EgressRanges  []string  `json:"egress_ranges"`
}

// PeerRouteInfo - struct for peer info for an ext. client
type PeerRouteInfo struct {
	PeerAddr net.IPNet `json:"peer_addr" yaml:"peer_addr"`
	PeerKey  string    `json:"peer_key" yaml:"peer_key"`
	Allow    bool      `json:"allow" yaml:"allow"`
	ID       string    `json:"id,omitempty" yaml:"id,omitempty"`
}

// ExtClientInfo - struct for ext. client and it's peers
type ExtClientInfo struct {
	IngGwAddr   net.IPNet                `json:"ingress_gw_addr" yaml:"ingress_gw_addr"`
	Network     net.IPNet                `json:"network" yaml:"network"`
	Masquerade  bool                     `json:"masquerade" yaml:"masquerade"`
	ExtPeerAddr net.IPNet                `json:"ext_peer_addr" yaml:"ext_peer_addr"`
	ExtPeerKey  string                   `json:"ext_peer_key" yaml:"ext_peer_key"`
	Peers       map[string]PeerRouteInfo `json:"peers" yaml:"peers"`
}

// KeyUpdate - key update struct
type KeyUpdate struct {
	Network   string `json:"network" bson:"network"`
	Interface string `json:"interface" bson:"interface"`
}

// FwUpdate - struct for firewall updates
type FwUpdate struct {
	AllowAll        bool                   `json:"allow_all"`
	AllowedNetworks []net.IPNet            `json:"networks"`
	IsEgressGw      bool                   `json:"is_egress_gw"`
	IsIngressGw     bool                   `json:"is_ingress_gw"`
	EgressInfo      map[string]EgressInfo  `json:"egress_info"`
	IngressInfo     map[string]IngressInfo `json:"ingress_info"`
	AclRules        map[string]AclRule     `json:"acl_rules"`
}

// FailOverMeReq - struct for failover req
type FailOverMeReq struct {
	NodeID string `json:"node_id"`
}
