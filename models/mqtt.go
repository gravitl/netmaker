package models

import "golang.zx2c4.com/wireguard/wgctrl/wgtypes"

type PeerUpdate struct {
	Network string
	Peers   []wgtypes.PeerConfig
}

type KeyUpdate struct {
	Network   string
	Interface string
}
