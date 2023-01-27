package models

import (
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
	ServerVersion string                 `json:"serverversion" bson:"serverversion" yaml:"serverversion"`
	ServerAddrs   []ServerAddr           `json:"serveraddrs" bson:"serveraddrs" yaml:"serveraddrs"`
	Network       map[string]NetworkInfo `json:"network" bson:"network" yaml:"network"`
	Peers         []wgtypes.PeerConfig   `json:"peers" bson:"peers" yaml:"peers"`
	PeerIDs       HostPeerMap            `json:"peerids" bson:"peerids" yaml:"peerids"`
	ProxyUpdate   ProxyManagerPayload    `json:"proxy_update" bson:"proxy_update" yaml:"proxy_update"`
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
