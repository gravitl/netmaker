package models

import "golang.zx2c4.com/wireguard/wgctrl/wgtypes"

// PeerUpdate - struct
type PeerUpdate struct {
	Network     string               `json:"network" bson:"network" yaml:"network"`
	ServerAddrs []ServerAddr         `json:"serveraddrs" bson:"serveraddrs" yaml:"serveraddrs"`
	Peers       []wgtypes.PeerConfig `json:"peers" bson:"peers" yaml:"peers"`
}

// KeyUpdate - key update struct
type KeyUpdate struct {
	Network   string `json:"network" bson:"network"`
	Interface string `json:"interface" bson:"interface"`
}

// RangeUpdate  - structure for network range updates
type RangeUpdate struct {
	Node  Node
	Peers PeerUpdate
}
