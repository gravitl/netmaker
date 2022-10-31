package proxy

import (
	"context"
	"errors"
	"fmt"
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
	LocalKey     string
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

func GetInterfaceIpv4Addr(interfaceName string) (addr string, err error) {
	var (
		ief      *net.Interface
		addrs    []net.Addr
		ipv4Addr net.IP
	)
	if ief, err = net.InterfaceByName(interfaceName); err != nil { // get interface
		return
	}
	if addrs, err = ief.Addrs(); err != nil { // get addresses
		return
	}
	for _, addr := range addrs { // get ipv4 address
		if ipv4Addr = addr.(*net.IPNet).IP.To4(); ipv4Addr != nil {
			break
		}
	}
	if ipv4Addr == nil {
		return "", errors.New(fmt.Sprintf("interface %s don't have an ipv4 address\n", interfaceName))
	}
	return ipv4Addr.String(), nil
}
