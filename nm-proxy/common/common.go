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
	IsAttachedExtClient bool
	IngressGateWay      *net.UDPAddr
}

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
	RemoteConn *net.UDPAddr
	LocalConn  net.Conn
}

type RemotePeer struct {
	PeerKey   string
	Interface string
	Endpoint  *net.UDPAddr
}

var WgIFaceMap = make(map[string]map[string]*Conn)

var PeerKeyHashMap = make(map[string]RemotePeer)

var WgIfaceKeyMap = make(map[string]struct{})

var RelayPeerMap = make(map[string]map[string]RemotePeer)

var ExtClientsMap = make(map[string]RemotePeer)

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
