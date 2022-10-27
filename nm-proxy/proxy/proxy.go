package proxy

import (
	"context"
	"net"

	"github.com/gravitl/netmaker/nm-proxy/wg"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	defaultBodySize = 10000
	defaultPort     = 51722
)

type Config struct {
	Port         int
	BodySize     int
	Addr         string
	RemoteKey    string
	WgInterface  *wg.WGIface
	AllowedIps   []net.IPNet
	PreSharedKey *wgtypes.Key
}

// Proxy -  WireguardProxy proxies
type Proxy struct {
	Ctx    context.Context
	Cancel context.CancelFunc

	Config     Config
	RemoteConn net.Conn
	LocalConn  net.Conn
}
