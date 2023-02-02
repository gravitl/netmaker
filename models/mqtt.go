package models

import (
	"net"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// PeerUpdate - struct
type PeerUpdate struct {
	Network       string               `json:"network" bson:"network" yaml:"network"`
	ServerVersion string               `json:"serverversion" bson:"serverversion" yaml:"serverversion"`
	ServerAddrs   []ServerAddr         `json:"serveraddrs" bson:"serveraddrs" yaml:"serveraddrs"`
	Peers         []wgtypes.PeerConfig `json:"peers" bson:"peers" yaml:"peers"`
	DNS           string               `json:"dns" bson:"dns" yaml:"dns"`
	PeerIDs       PeerMap              `json:"peerids" bson:"peerids" yaml:"peerids"`
	ProxyUpdate   ProxyManagerPayload  `json:"proxy_update" bson:"proxy_update" yaml:"proxy_update"`
}

// HostPeerUpdate - struct for host peer updates
type HostPeerUpdate struct {
	Host          Host                   `json:"host" bson:"host" yaml:"host"`
	Server        string                 `json:"server" bson:"server" yaml:"server"`
	ServerVersion string                 `json:"serverversion" bson:"serverversion" yaml:"serverversion"`
	ServerAddrs   []ServerAddr           `json:"serveraddrs" bson:"serveraddrs" yaml:"serveraddrs"`
	Network       map[string]NetworkInfo `json:"network" bson:"network" yaml:"network"`
	Peers         []wgtypes.PeerConfig   `json:"peers" bson:"peers" yaml:"peers"`
	PeerIDs       HostPeerMap            `json:"peerids" bson:"peerids" yaml:"peerids"`
	ProxyUpdate   ProxyManagerPayload    `json:"proxy_update" bson:"proxy_update" yaml:"proxy_update"`
	EgressInfo    map[string]EgressInfo  `json:"egress_info" bson:"egress_info" yaml:"egress_info"` // map key is node ID
	IngressInfo   IngressInfo            `json:"ingress_info" bson:"ext_peers" yaml:"ext_peers"`
}

// IngressInfo - struct for ingress info
type IngressInfo struct {
	ExtPeers map[string]ExtClientInfo `json:"ext_peers" yaml:"ext_peers"`
}

type EgressInfo struct {
	EgressID     string                   `json:"egress_id" yaml:"egress_id"`
	EgressGwAddr net.IPNet                `json:"egress_gw_addr" yaml:"egress_gw_addr"`
	GwPeers      map[string]PeerRouteInfo `json:"gateway_peers" yaml:"gateway_peers"`
	EgressGWCfg  EgressGatewayRequest     `json:"egress_gateway_cfg" yaml:"egress_gateway_cfg"`
}

// PeerRouteInfo - struct for peer info for an ext. client
type PeerRouteInfo struct {
	PeerAddr net.IPNet `json:"peer_addr" yaml:"peer_addr"`
	PeerKey  string    `json:"peer_key" yaml:"peer_key"`
	Allow    bool      `json:"allow" yaml:"allow"`
}

// ExtClientInfo - struct for ext. client and it's peers
type ExtClientInfo struct {
	IngGwAddr   net.IPNet                `json:"ingress_gw_addr" yaml:"ingress_gw_addr"`
	Masquerade  bool                     `json:"masquerade" yaml:"masquerade"`
	ExtPeerAddr net.IPNet                `json:"ext_peer_addr" yaml:"ext_peer_addr"`
	ExtPeerKey  string                   `json:"ext_peer_key" yaml:"ext_peer_key"`
	Peers       map[string]PeerRouteInfo `json:"peers" yaml:"peers"`
}

// NetworkInfo - struct for network info
type NetworkInfo struct {
	DNS string `json:"dns" bson:"dns" yaml:"dns"`
}

// KeyUpdate - key update struct
type KeyUpdate struct {
	Network   string `json:"network" bson:"network"`
	Interface string `json:"interface" bson:"interface"`
}
