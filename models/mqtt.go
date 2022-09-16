package models

import "golang.zx2c4.com/wireguard/wgctrl/wgtypes"

// PeerUpdate - struct
type PeerUpdate struct {
	Network       string               `json:"network" bson:"network" yaml:"network"`
	ServerVersion string               `json:"serverversion" bson:"serverversion" yaml:"serverversion"`
	ServerAddrs   []ServerAddr         `json:"serveraddrs" bson:"serveraddrs" yaml:"serveraddrs"`
	Peers         []wgtypes.PeerConfig `json:"peers" bson:"peers" yaml:"peers"`
	DNS           string               `json:"dns" bson:"dns" yaml:"dns"`
	PeerIDs       PeerMap              `json:"peerids" bson:"peerids" yaml:"peerids"`
}

// KeyUpdate - key update struct
type KeyUpdate struct {
	Network   string `json:"network" bson:"network"`
	Interface string `json:"interface" bson:"interface"`
}
