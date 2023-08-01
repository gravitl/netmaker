package models

import (
	"net"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// HostPeerUpdate - struct for host peer updates
type HostPeerUpdate struct {
	Host              Host                 `json:"host" bson:"host" yaml:"host"`
	NodeAddrs         []net.IPNet          `json:"nodes_addrs" yaml:"nodes_addrs"`
	Server            string               `json:"server" bson:"server" yaml:"server"`
	ServerVersion     string               `json:"serverversion" bson:"serverversion" yaml:"serverversion"`
	ServerAddrs       []ServerAddr         `json:"serveraddrs" bson:"serveraddrs" yaml:"serveraddrs"`
	NodePeers         []wgtypes.PeerConfig `json:"peers" bson:"peers" yaml:"peers"`
	Peers             []wgtypes.PeerConfig
	PeerIDs           PeerMap               `json:"peerids" bson:"peerids" yaml:"peerids"`
	EndpointDetection bool                  `json:"endpointdetection" yaml:"endpointdetection"`
	HostNetworkInfo   HostInfoMap           `json:"host_network_info,omitempty" bson:"host_network_info,omitempty" yaml:"host_network_info,omitempty"`
	EgressRoutes      []EgressNetworkRoutes `json:"egress_network_routes"`
	FwUpdate          FwUpdate              `json:"fw_update"`
}

// IngressInfo - struct for ingress info
type IngressInfo struct {
	ExtPeers     map[string]ExtClientInfo `json:"ext_peers" yaml:"ext_peers"`
	EgressRanges []string                 `json:"egress_ranges" yaml:"egress_ranges"`
}

// EgressInfo - struct for egress info
type EgressInfo struct {
	EgressID     string               `json:"egress_id" yaml:"egress_id"`
	Network      net.IPNet            `json:"network" yaml:"network"`
	EgressGwAddr net.IPNet            `json:"egress_gw_addr" yaml:"egress_gw_addr"`
	EgressGWCfg  EgressGatewayRequest `json:"egress_gateway_cfg" yaml:"egress_gateway_cfg"`
}

// EgressNetworkRoutes - struct for egress network routes for adding routes to peer's interface
type EgressNetworkRoutes struct {
	NodeAddr     net.IPNet `json:"node_addr"`
	EgressRanges []string  `json:"egress_ranges"`
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
	IsEgressGw bool                  `json:"is_egress_gw"`
	EgressInfo map[string]EgressInfo `json:"egress_info"`
}
