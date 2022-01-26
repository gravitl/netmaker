package models

import "golang.zx2c4.com/wireguard/wgctrl/wgtypes"

// PeerUpdate - struct
type PeerUpdate struct {
	Network     string               `json:"network" bson:"network"`
	ServerAddrs []string             `json:"serversaddrs" bson:"serversaddrs"`
	Peers       []wgtypes.PeerConfig `json:"peers" bson:"peers"`
}

// KeyUpdate - key update struct
type KeyUpdate struct {
	Network   string `json:"network" bson:"network"`
	Interface string `json:"interface" bson:"interface"`
}
