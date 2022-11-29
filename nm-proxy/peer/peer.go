package peer

import (
	"errors"
	"fmt"
	"log"
	"net"

	"github.com/gravitl/netmaker/nm-proxy/common"
	"github.com/gravitl/netmaker/nm-proxy/models"
	"github.com/gravitl/netmaker/nm-proxy/proxy"
	"github.com/gravitl/netmaker/nm-proxy/wg"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func AddNewPeer(wgInterface *wg.WGIface, peer *wgtypes.PeerConfig, peerAddr string,
	isRelayed, isExtClient, isAttachedExtClient bool, relayTo *net.UDPAddr) error {

	c := proxy.Config{
		Port:        peer.Endpoint.Port,
		LocalKey:    wgInterface.Device.PublicKey,
		RemoteKey:   peer.PublicKey,
		WgInterface: wgInterface,
		IsExtClient: isExtClient,
		PeerConf:    peer,
	}
	p := proxy.NewProxy(c)
	peerPort := models.NmProxyPort
	if isExtClient && isAttachedExtClient {
		peerPort = peer.Endpoint.Port

	}
	peerEndpoint := peer.Endpoint.IP.String()
	if isRelayed {
		//go server.NmProxyServer.KeepAlive(peer.Endpoint.IP.String(), common.NmProxyPort)
		if relayTo == nil {
			return errors.New("relay endpoint is nil")
		}
		peerEndpoint = relayTo.IP.String()
	}

	remoteConn, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", peerEndpoint, peerPort))
	if err != nil {
		return err
	}
	log.Printf("----> Established Remote Conn with RPeer: %s, ----> RAddr: %s", peer.PublicKey, remoteConn.String())

	// if !(isExtClient && isAttachedExtClient) {
	log.Printf("Starting proxy for Peer: %s\n", peer.PublicKey.String())
	err = p.Start(remoteConn)
	if err != nil {
		return err
	}
	// } else {
	// 	log.Println("Not Starting Proxy for Attached ExtClient...")
	// }

	connConf := models.ConnConfig{
		Key:                 peer.PublicKey,
		IsRelayed:           isRelayed,
		RelayedEndpoint:     relayTo,
		IsAttachedExtClient: isAttachedExtClient,
		PeerConf:            peer,
		StopConn:            p.Cancel,
		RemoteConn:          remoteConn,
		LocalConn:           p.LocalConn,
	}

	common.WgIfaceMap.PeerMap[peer.PublicKey.String()] = &connConf

	return nil
}
