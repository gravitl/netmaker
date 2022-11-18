package common

import (
	"context"
	"log"
	"net"
	"os/exec"
	"strings"

	"github.com/gravitl/netmaker/nm-proxy/wg"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var IsHostNetwork bool
var IsRelay bool
var IsIngressGateway bool
var IsRelayed bool

const (
	NmProxyPort = 51722
	DefaultCIDR = "127.0.0.1/8"
)

type Conn struct {
	Config ConnConfig
	Proxy  Proxy
}

// ConnConfig is a peer Connection configuration
type ConnConfig struct {

	// Key is a public key of a remote peer
	Key string
	// LocalKey is a public key of a local peer
	LocalKey            string
	LocalWgPort         int
	RemoteProxyIP       net.IP
	RemoteWgPort        int
	RemoteProxyPort     int
	IsExtClient         bool
	IsRelayed           bool
	RelayedEndpoint     *net.UDPAddr
	IsAttachedExtClient bool
	IngressGateWay      *net.UDPAddr
}

type Config struct {
	Port        int
	BodySize    int
	Addr        string
	RemoteKey   string
	LocalKey    string
	WgInterface *wg.WGIface
	PeerConf    *wgtypes.PeerConfig
}

// Proxy -  WireguardProxy proxies
type Proxy struct {
	Ctx    context.Context
	Cancel context.CancelFunc

	Config     Config
	RemoteConn *net.UDPAddr
	LocalConn  net.Conn
}

type RemotePeer struct {
	PeerKey             string
	Interface           string
	Endpoint            *net.UDPAddr
	IsExtClient         bool
	IsAttachedExtClient bool
}

type WgIfaceConf struct {
	Iface   *wgtypes.Device
	PeerMap map[string]*Conn
}

var WgIFaceMap = make(map[string]WgIfaceConf)

var PeerKeyHashMap = make(map[string]RemotePeer)

var WgIfaceKeyMap = make(map[string]RemotePeer)

var RelayPeerMap = make(map[string]map[string]RemotePeer)

var ExtClientsWaitTh = make(map[string][]context.CancelFunc)

var PeerAddrMap = make(map[string]map[string]*Conn)

// RunCmd - runs a local command
func RunCmd(command string, printerr bool) (string, error) {
	args := strings.Fields(command)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Wait()
	out, err := cmd.CombinedOutput()
	if err != nil && printerr {
		log.Println("error running command: ", command)
		log.Println(strings.TrimSuffix(string(out), "\n"))
	}
	return string(out), err
}
