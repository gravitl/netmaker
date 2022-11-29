package models

import (
	"context"
	"net"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	NmProxyPort = 51722
	DefaultCIDR = "127.0.0.1/8"
)

// ConnConfig is a peer Connection configuration
type ConnConfig struct {

	// Key is a public key of a remote peer
	Key                 wgtypes.Key
	IsExtClient         bool
	IsRelayed           bool
	RelayedEndpoint     *net.UDPAddr
	IsAttachedExtClient bool
	PeerConf            *wgtypes.PeerConfig
	StopConn            context.CancelFunc
	RemoteConn          *net.UDPAddr
	LocalConn           net.Conn
}

type RemotePeer struct {
	PeerKey             string
	Interface           string
	Endpoint            *net.UDPAddr
	IsExtClient         bool
	IsAttachedExtClient bool
}

type ExtClientPeer struct {
	CancelFunc context.CancelFunc
	CommChan   chan *net.UDPAddr
}

type WgIfaceConf struct {
	Iface        *wgtypes.Device
	IfaceKeyHash string
	PeerMap      map[string]*ConnConfig
}
