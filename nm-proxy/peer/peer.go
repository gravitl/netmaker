package peer

import (
	"crypto/md5"
	"fmt"
	"log"
	"net"

	"github.com/gravitl/netmaker/nm-proxy/common"
	"github.com/gravitl/netmaker/nm-proxy/proxy"
	"github.com/gravitl/netmaker/nm-proxy/server"
	"github.com/gravitl/netmaker/nm-proxy/wg"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type Conn struct {
	Config ConnConfig
	Proxy  proxy.Proxy
}

// ConnConfig is a peer Connection configuration
type ConnConfig struct {

	// Key is a public key of a remote peer
	Key string
	// LocalKey is a public key of a local peer
	LocalKey string

	ProxyConfig     proxy.Config
	AllowedIPs      string
	LocalWgPort     int
	RemoteProxyIP   net.IP
	RemoteWgPort    int
	RemoteProxyPort int
}

func AddNewPeer(wgInterface *wg.WGIface, peer *wgtypes.PeerConfig, isRelayed bool, relayTo *net.UDPAddr) error {

	c := proxy.Config{
		Port:        peer.Endpoint.Port,
		LocalKey:    wgInterface.Device.PublicKey.String(),
		RemoteKey:   peer.PublicKey.String(),
		WgInterface: wgInterface,
		AllowedIps:  peer.AllowedIPs,
	}
	p := proxy.NewProxy(c)

	peerEndpoint := peer.Endpoint.IP.String()
	if isRelayed {
		go server.NmProxyServer.KeepAlive(peer.Endpoint.IP.String(), common.NmProxyPort)
		peerEndpoint = relayTo.IP.String()
	}
	remoteConn, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", peerEndpoint, common.NmProxyPort))
	if err != nil {
		return err
	}
	log.Printf("----> Established Remote Conn with RPeer: %s, ----> RAddr: %s", peer.PublicKey, remoteConn.String())
	log.Printf("Starting proxy for Peer: %s\n", peer.PublicKey.String())
	err = p.Start(remoteConn)
	if err != nil {
		return err
	}
	connConf := common.ConnConfig{
		Key:             peer.PublicKey.String(),
		LocalKey:        wgInterface.Device.PublicKey.String(),
		LocalWgPort:     wgInterface.Device.ListenPort,
		RemoteProxyIP:   net.ParseIP(peer.Endpoint.IP.String()),
		RemoteWgPort:    peer.Endpoint.Port,
		RemoteProxyPort: common.NmProxyPort,
	}

	peerProxy := common.Proxy{
		Ctx:    p.Ctx,
		Cancel: p.Cancel,
		Config: common.Config{
			Port:        peer.Endpoint.Port,
			LocalKey:    wgInterface.Device.PublicKey.String(),
			RemoteKey:   peer.PublicKey.String(),
			WgInterface: wgInterface,
			AllowedIps:  peer.AllowedIPs,
		},

		RemoteConn: remoteConn,
		LocalConn:  p.LocalConn,
	}
	if isRelayed {
		connConf.RemoteProxyIP = relayTo.IP
	}
	peerConn := common.Conn{
		Config: connConf,
		Proxy:  peerProxy,
	}
	if _, ok := common.WgIFaceMap[wgInterface.Name]; ok {
		common.WgIFaceMap[wgInterface.Name][peer.PublicKey.String()] = &peerConn
	} else {
		common.WgIFaceMap[wgInterface.Name] = make(map[string]*common.Conn)
		common.WgIFaceMap[wgInterface.Name][peer.PublicKey.String()] = &peerConn
	}
	common.WgIfaceKeyMap[fmt.Sprintf("%x", md5.Sum([]byte(wgInterface.Device.PublicKey.String())))] = struct{}{}
	return nil
}
