package models

import (
	proxy_models "github.com/gravitl/netclient/nmproxy/models"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// PeerUpdate - struct
type PeerUpdate struct {
	Network       string                           `json:"network" bson:"network" yaml:"network"`
	ServerVersion string                           `json:"serverversion" bson:"serverversion" yaml:"serverversion"`
	ServerAddrs   []ServerAddr                     `json:"serveraddrs" bson:"serveraddrs" yaml:"serveraddrs"`
	Peers         []wgtypes.PeerConfig             `json:"peers" bson:"peers" yaml:"peers"`
	DNS           string                           `json:"dns" bson:"dns" yaml:"dns"`
	PeerIDs       PeerMap                          `json:"peerids" bson:"peerids" yaml:"peerids"`
	ProxyUpdate   proxy_models.ProxyManagerPayload `json:"proxy_update" bson:"proxy_update" yaml:"proxy_update"`
}

// HostPeerUpdate - struct for host peer updates
type HostPeerUpdate struct {
	Host          Host                             `json:"host" bson:"host" yaml:"host"`
	Server        string                           `json:"server" bson:"server" yaml:"server"`
	ServerVersion string                           `json:"serverversion" bson:"serverversion" yaml:"serverversion"`
	ServerAddrs   []ServerAddr                     `json:"serveraddrs" bson:"serveraddrs" yaml:"serveraddrs"`
	Network       map[string]NetworkInfo           `json:"network" bson:"network" yaml:"network"`
	Peers         []wgtypes.PeerConfig             `json:"peers" bson:"peers" yaml:"peers"`
	PeerIDs       HostPeerMap                      `json:"peerids" bson:"peerids" yaml:"peerids"`
	ProxyUpdate   proxy_models.ProxyManagerPayload `json:"proxy_update" bson:"proxy_update" yaml:"proxy_update"`
	IngressInfo   IngressInfo                      `json:"ingress_info" bson:"ext_peers" yaml:"ext_peers"`
}

type IngressInfo struct {
	ExtPeers map[string]ExtClientInfo
}

type PeerExtInfo struct {
	PeerConfig wgtypes.PeerConfig
	Allow      bool
}

type ExtClientInfo struct {
	ExtPeer wgtypes.PeerConfig
	Peers   map[string]PeerExtInfo
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
