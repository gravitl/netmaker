package peer

import (
	"errors"
	"log"
	"net"
	"time"

	"github.com/gravitl/netmaker/nm-proxy/common"
	"github.com/gravitl/netmaker/nm-proxy/metrics"
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
	c := proxy.Config{
		LocalKey:            wgInterface.Device.PublicKey,
		RemoteKey:           peer.PublicKey,
		WgInterface:         wgInterface,
		IsExtClient:         isExtClient,
		PeerConf:            peer,
		PersistentKeepalive: peer.PersistentKeepaliveInterval,
		RecieverChan:        make(chan []byte, 100),
		MetricsCh:           make(chan metrics.MetricsPayload, 30),
	}
	p := proxy.NewProxy(c)
	peerPort := models.NmProxyPort
	if isExtClient && isAttachedExtClient {
		peerPort = peer.Endpoint.Port

	}
	peerEndpoint := peer.Endpoint.IP
	if isRelayed {
		//go server.NmProxyServer.KeepAlive(peer.Endpoint.IP.String(), common.NmProxyPort)
		if relayTo == nil {
			return errors.New("relay endpoint is nil")
		}
		peerEndpoint = relayTo.IP
	}
	p.Config.PeerIp = peerEndpoint
	p.Config.PeerPort = uint32(peerPort)

	log.Printf("Starting proxy for Peer: %s\n", peer.PublicKey.String())
	lAddr, rAddr, err := p.Start()
	if err != nil {
		return err
	}

	connConf := models.ConnConfig{
		Key:                 peer.PublicKey,
		IsRelayed:           isRelayed,
		RelayedEndpoint:     relayTo,
		IsAttachedExtClient: isAttachedExtClient,
		PeerConf:            peer,
		StopConn:            p.Close,
		ResetConn:           p.Reset,
		RemoteConnAddr:      rAddr,
		LocalConnAddr:       lAddr,
		RecieverChan:        p.Config.RecieverChan,
		PeerListenPort:      p.Config.PeerPort,
	}

	common.WgIfaceMap.PeerMap[peer.PublicKey.String()] = &connConf

	return nil
}
