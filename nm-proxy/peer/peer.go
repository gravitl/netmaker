package peer

import (
	"crypto/md5"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/gravitl/netmaker/nm-proxy/common"
	"github.com/gravitl/netmaker/nm-proxy/models"
	"github.com/gravitl/netmaker/nm-proxy/proxy"
	"github.com/gravitl/netmaker/nm-proxy/wg"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func AddNewPeer(wgInterface *wg.WGIface, peer *wgtypes.PeerConfig, peerAddr string,
	isRelayed, isExtClient, isAttachedExtClient bool, relayTo *net.UDPAddr) error {
	if peer.PersistentKeepaliveInterval == nil {
		d := time.Second * 25
		peer.PersistentKeepaliveInterval = &d
	}
	c := models.ProxyConfig{
		LocalKey:            wgInterface.Device.PublicKey,
		RemoteKey:           peer.PublicKey,
		WgInterface:         wgInterface,
		IsExtClient:         isExtClient,
		PeerConf:            peer,
		PersistentKeepalive: peer.PersistentKeepaliveInterval,
		RecieverChan:        make(chan []byte, 1000),
	}
	p := proxy.NewProxy(c)
	peerPort := models.NmProxyPort
	if isExtClient && isAttachedExtClient {
		peerPort = peer.Endpoint.Port

	}
	peerEndpointIP := peer.Endpoint.IP
	if isRelayed {
		//go server.NmProxyServer.KeepAlive(peer.Endpoint.IP.String(), common.NmProxyPort)
		if relayTo == nil {
			return errors.New("relay endpoint is nil")
		}
		peerEndpointIP = relayTo.IP
	}
	peerEndpoint, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", peerEndpointIP, peerPort))
	if err != nil {
		return err
	}
	p.Config.PeerEndpoint = peerEndpoint

	log.Printf("Starting proxy for Peer: %s\n", peer.PublicKey.String())
	err = p.Start()
	if err != nil {
		return err
	}

	connConf := models.Conn{
		Mutex:               &sync.RWMutex{},
		Key:                 peer.PublicKey,
		IsRelayed:           isRelayed,
		RelayedEndpoint:     relayTo,
		IsAttachedExtClient: isAttachedExtClient,
		Config:              p.Config,
		StopConn:            p.Close,
		ResetConn:           p.Reset,
		LocalConn:           p.LocalConn,
	}

	common.WgIfaceMap.PeerMap[peer.PublicKey.String()] = &connConf

	common.PeerKeyHashMap[fmt.Sprintf("%x", md5.Sum([]byte(peer.PublicKey.String())))] = models.RemotePeer{
		Interface:           wgInterface.Name,
		PeerKey:             peer.PublicKey.String(),
		IsExtClient:         isExtClient,
		Endpoint:            peerEndpoint,
		IsAttachedExtClient: isAttachedExtClient,
		LocalConn:           p.LocalConn,
	}
	return nil
}
