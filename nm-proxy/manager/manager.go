package manager

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"log"
	"net"
	"reflect"
	"runtime"
	"time"

	"github.com/gravitl/netmaker/nm-proxy/common"
	peerpkg "github.com/gravitl/netmaker/nm-proxy/peer"
	"github.com/gravitl/netmaker/nm-proxy/proxy"
	"github.com/gravitl/netmaker/nm-proxy/wg"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

/*
TODO:-
	1. ON Ingress node
		--> for attached ext clients
			-> start sniffer (will recieve pkts from ext clients (add ebf filter to listen on only ext traffic) if not intended to the interface forward it.)
			-> start remote conn after endpoint is updated
		-->
*/
var sent bool

type ProxyAction string

type ManagerPayload struct {
	InterfaceName   string                 `json:"interface_name"`
	WgAddr          string                 `json:"wg_addr"`
	Peers           []wgtypes.PeerConfig   `json:"peers"`
	PeerMap         map[string]PeerConf    `json:"peer_map"`
	IsRelayed       bool                   `json:"is_relayed"`
	IsIngress       bool                   `json:"is_ingress"`
	RelayedTo       *net.UDPAddr           `json:"relayed_to"`
	IsRelay         bool                   `json:"is_relay"`
	RelayedPeerConf map[string]RelayedConf `json:"relayed_conf"`
}

type RelayedConf struct {
	RelayedPeerEndpoint *net.UDPAddr         `json:"relayed_peer_endpoint"`
	RelayedPeerPubKey   string               `json:"relayed_peer_pub_key"`
	Peers               []wgtypes.PeerConfig `json:"relayed_peers"`
}

type PeerConf struct {
	IsExtClient            bool         `json:"is_ext_client"`
	Address                string       `json:"address"`
	IsAttachedExtClient    bool         `json:"is_attached_ext_client"`
	IngressGatewayEndPoint *net.UDPAddr `json:"ingress_gateway_endpoint"`
	IsRelayed              bool         `json:"is_relayed"`
	RelayedTo              *net.UDPAddr `json:"relayed_to"`
	Proxy                  bool         `json:"proxy"`
}

const (
	AddInterface    ProxyAction = "ADD_INTERFACE"
	DeleteInterface ProxyAction = "DELETE_INTERFACE"
)

type ManagerAction struct {
	Action  ProxyAction
	Payload ManagerPayload
}

func StartProxyManager(manageChan chan *ManagerAction) {
	for {

		select {
		case mI := <-manageChan:
			log.Printf("-------> PROXY-MANAGER: %+v\n", mI)
			switch mI.Action {
			case AddInterface:

				mI.SetIngressGateway()
				err := mI.AddInterfaceToProxy()
				if err != nil {
					log.Printf("failed to add interface: [%s] to proxy: %v\n  ", mI.Payload.InterfaceName, err)
				}
			case DeleteInterface:
				mI.DeleteInterface()
			}

		}
	}
}

func (m *ManagerAction) DeleteInterface() {
	var err error
	if runtime.GOOS == "darwin" {
		m.Payload.InterfaceName, err = wg.GetRealIface(m.Payload.InterfaceName)
		if err != nil {
			log.Println("failed to get real iface: ", err)
			return
		}
	}
	if wgProxyConf, ok := common.WgIFaceMap[m.Payload.InterfaceName]; ok {
		cleanUpInterface(wgProxyConf)
	}

}

func (m *ManagerAction) RelayUpdate() {
	common.IsRelay = m.Payload.IsRelay
}

func (m *ManagerAction) SetIngressGateway() {
	common.IsIngressGateway = m.Payload.IsIngress

}

func (m *ManagerAction) RelayPeers() {
	common.IsRelay = true
	for relayedNodePubKey, relayedNodeConf := range m.Payload.RelayedPeerConf {
		relayedNodePubKeyHash := fmt.Sprintf("%x", md5.Sum([]byte(relayedNodePubKey)))
		if _, ok := common.RelayPeerMap[relayedNodePubKeyHash]; !ok {
			common.RelayPeerMap[relayedNodePubKeyHash] = make(map[string]common.RemotePeer)
		}
		for _, peer := range relayedNodeConf.Peers {
			if peer.Endpoint != nil {
				peer.Endpoint.Port = common.NmProxyPort
				remotePeerKeyHash := fmt.Sprintf("%x", md5.Sum([]byte(peer.PublicKey.String())))
				common.RelayPeerMap[relayedNodePubKeyHash][remotePeerKeyHash] = common.RemotePeer{
					Endpoint: peer.Endpoint,
				}
			}

		}
		relayedNodeConf.RelayedPeerEndpoint.Port = common.NmProxyPort
		common.RelayPeerMap[relayedNodePubKeyHash][relayedNodePubKeyHash] = common.RemotePeer{
			Endpoint: relayedNodeConf.RelayedPeerEndpoint,
		}

	}
}

func cleanUpInterface(ifaceConf common.WgIfaceConf) {
	log.Println("########------------>  CLEANING UP: ", ifaceConf.Iface.Name)
	for _, peerI := range ifaceConf.PeerMap {
		peerI.Proxy.Cancel()
	}
	delete(common.WgIFaceMap, ifaceConf.Iface.Name)
}

func (m *ManagerAction) processPayload() (*wg.WGIface, error) {
	var err error
	var wgIface *wg.WGIface
	if m.Payload.InterfaceName == "" {
		return nil, errors.New("interface cannot be empty")
	}
	if len(m.Payload.Peers) == 0 {
		return nil, errors.New("no peers to add")
	}

	if runtime.GOOS == "darwin" {
		m.Payload.InterfaceName, err = wg.GetRealIface(m.Payload.InterfaceName)
		if err != nil {
			log.Println("failed to get real iface: ", err)
		}
	}
	wgIface, err = wg.NewWGIFace(m.Payload.InterfaceName, "127.0.0.1/32", wg.DefaultMTU)
	if err != nil {
		log.Println("Failed init new interface: ", err)
		return nil, err
	}
	var wgProxyConf common.WgIfaceConf
	var ok bool
	if wgProxyConf, ok = common.WgIFaceMap[m.Payload.InterfaceName]; !ok {
		for i := len(m.Payload.Peers) - 1; i >= 0; i-- {
			if !m.Payload.PeerMap[m.Payload.Peers[i].PublicKey.String()].Proxy {
				log.Println("-----------> skipping peer, proxy is off: ", m.Payload.Peers[i].PublicKey)
				if err := wgIface.Update(m.Payload.Peers[i], false); err != nil {
					log.Println("falied to update peer: ", err)
				}
				m.Payload.Peers = append(m.Payload.Peers[:i], m.Payload.Peers[i+1:]...)
			}
		}
		return wgIface, nil
	}
	if m.Payload.IsRelay {
		m.RelayPeers()
	}
	common.IsRelay = m.Payload.IsRelay
	// check if node is getting relayed
	if common.IsRelayed != m.Payload.IsRelayed {
		common.IsRelayed = m.Payload.IsRelayed
		cleanUpInterface(wgProxyConf)
		return wgIface, nil
	}

	// sync map with wg device config
	// check if listen port has changed
	if wgIface.Device.ListenPort != wgProxyConf.Iface.ListenPort {
		// reset proxy for this interface
		cleanUpInterface(wgProxyConf)
		return wgIface, nil
	}
	// check device conf different from proxy
	//wgProxyConf.Iface = wgIface.Device
	for i := len(m.Payload.Peers) - 1; i >= 0; i-- {

		if currentPeer, ok := wgProxyConf.PeerMap[m.Payload.Peers[i].PublicKey.String()]; ok {

			// handles ext clients
			if common.IsIngressGateway && m.Payload.PeerMap[m.Payload.Peers[i].PublicKey.String()].IsAttachedExtClient {
				// check if sniffer already exists otherwise start one
				if _, ok := common.ExtClientsWaitTh[m.Payload.Peers[i].PublicKey.String()]; ok {
					log.Println("Ext client th already exists....,removing")
					m.Payload.Peers = append(m.Payload.Peers[:i], m.Payload.Peers[i+1:]...)

				}
				log.Println("----> Skip Processing ExtClient: ", m.Payload.Peers[i].PublicKey.String())
				continue
			}
			// check if proxy is off for the peer
			if !m.Payload.PeerMap[m.Payload.Peers[i].PublicKey.String()].Proxy {

				// cleanup proxy connections for the peer
				currentPeer.Proxy.Cancel()
				time.Sleep(time.Second * 3)
				delete(wgProxyConf.PeerMap, currentPeer.Config.Key)
				// update the peer with actual endpoint
				if err := wgIface.Update(m.Payload.Peers[i], false); err != nil {
					log.Println("falied to update peer: ", err)
				}
				m.Payload.Peers = append(m.Payload.Peers[:i], m.Payload.Peers[i+1:]...)
				continue

			}
			// check if peer is not connected to proxy
			devPeer, err := wg.GetPeer(m.Payload.InterfaceName, currentPeer.Config.Key)
			if err == nil {
				log.Printf("---------> COMAPRING ENDPOINT: DEV: %s, Proxy: %s", devPeer.Endpoint.String(), currentPeer.Proxy.LocalConn.LocalAddr().String())
				if devPeer.Endpoint.String() != currentPeer.Proxy.LocalConn.LocalAddr().String() {
					log.Println("---------> endpoint is not set to proxy: ", currentPeer.Config.Key)
					currentPeer.Proxy.Cancel()
					time.Sleep(time.Second * 3)
					delete(wgProxyConf.PeerMap, currentPeer.Config.Key)
					continue
				}
			}
			//check if peer is being relayed
			if currentPeer.Config.IsRelayed != m.Payload.PeerMap[m.Payload.Peers[i].PublicKey.String()].IsRelayed {
				log.Println("---------> peer relay status has been changed: ", currentPeer.Config.Key)
				currentPeer.Proxy.Cancel()
				time.Sleep(time.Second * 3)
				delete(wgProxyConf.PeerMap, currentPeer.Config.Key)
				continue
			}
			// check if relay endpoint has been changed
			if currentPeer.Config.RelayedEndpoint != nil &&
				m.Payload.PeerMap[m.Payload.Peers[i].PublicKey.String()].RelayedTo != nil &&
				currentPeer.Config.RelayedEndpoint.String() != m.Payload.PeerMap[m.Payload.Peers[i].PublicKey.String()].RelayedTo.String() {
				log.Println("---------> peer relay endpoint has been changed: ", currentPeer.Config.Key)
				currentPeer.Proxy.Cancel()
				time.Sleep(time.Second * 3)
				delete(wgProxyConf.PeerMap, currentPeer.Config.Key)
				continue
			}
			if !reflect.DeepEqual(m.Payload.Peers[i], *currentPeer.Proxy.Config.PeerConf) {
				if currentPeer.Proxy.RemoteConn.IP.String() != m.Payload.Peers[i].Endpoint.IP.String() {
					log.Println("----------> Resetting proxy for Peer: ", currentPeer.Config.Key, m.Payload.InterfaceName)
					currentPeer.Proxy.Cancel()
					time.Sleep(time.Second * 3)
					delete(wgProxyConf.PeerMap, currentPeer.Config.Key)

				} else {

					log.Println("----->##### Updating Peer on Interface: ", m.Payload.InterfaceName, currentPeer.Config.Key)
					updatePeerConf := m.Payload.Peers[i]
					localUdpAddr, err := net.ResolveUDPAddr("udp", currentPeer.Proxy.LocalConn.LocalAddr().String())
					if err == nil {
						updatePeerConf.Endpoint = localUdpAddr
					}
					if err := wgIface.Update(updatePeerConf, true); err != nil {
						log.Println("failed to update peer: ", currentPeer.Config.Key, err)
					}
					currentPeer.Proxy.Config.PeerConf = &m.Payload.Peers[i]
					wgProxyConf.PeerMap[currentPeer.Config.Key] = currentPeer
					// delete the peer from the list
					log.Println("-----------> deleting peer from list: ", m.Payload.Peers[i].PublicKey)
					m.Payload.Peers = append(m.Payload.Peers[:i], m.Payload.Peers[i+1:]...)

				}

			} else {
				// delete the peer from the list
				log.Println("-----------> No updates observed so deleting peer: ", m.Payload.Peers[i].PublicKey)
				m.Payload.Peers = append(m.Payload.Peers[:i], m.Payload.Peers[i+1:]...)
			}

		} else if !m.Payload.PeerMap[m.Payload.Peers[i].PublicKey.String()].Proxy && !m.Payload.PeerMap[m.Payload.Peers[i].PublicKey.String()].IsAttachedExtClient {
			log.Println("-----------> skipping peer, proxy is off: ", m.Payload.Peers[i].PublicKey)
			if err := wgIface.Update(m.Payload.Peers[i], false); err != nil {
				log.Println("falied to update peer: ", err)
			}
			m.Payload.Peers = append(m.Payload.Peers[:i], m.Payload.Peers[i+1:]...)
		}
	}
	// sync peer map with new update
	for _, currPeerI := range wgProxyConf.PeerMap {
		if _, ok := m.Payload.PeerMap[currPeerI.Config.Key]; !ok {
			currPeerI.Proxy.Cancel()
			time.Sleep(time.Second * 3)
			// delete peer from interface
			log.Println("CurrPeer Not Found, Deleting Peer from Interface: ", currPeerI.Config.Key)
			if err := wgIface.RemovePeer(currPeerI.Config.Key); err != nil {
				log.Println("failed to remove peer: ", currPeerI.Config.Key, err)
			}
			delete(wgProxyConf.PeerMap, currPeerI.Config.Key)

		}
	}

	// sync dev peers with new update

	common.WgIFaceMap[m.Payload.InterfaceName] = wgProxyConf

	// if peers, ok := common.WgIFaceMap[iface]; ok {
	// 	log.Println("########------------>  CLEANING UP: ", iface)
	// 	for _, peerI := range peers {
	// 		peerI.Proxy.Cancel()
	// 	}
	// }
	// delete(common.WgIFaceMap, iface)
	// delete(common.PeerAddrMap, iface)
	// if waitThs, ok := common.ExtClientsWaitTh[iface]; ok {
	// 	for _, cancelF := range waitThs {
	// 		cancelF()
	// 	}
	// 	delete(common.ExtClientsWaitTh, iface)
	// }

	log.Println("CLEANED UP..........")
	return wgIface, nil
}

func (m *ManagerAction) AddInterfaceToProxy() error {
	var err error

	wgInterface, err := m.processPayload()
	if err != nil {
		return err
	}

	log.Printf("wg: %+v\n", wgInterface)
	wgListenAddr, err := proxy.GetInterfaceListenAddr(wgInterface.Port)
	if err != nil {
		log.Println("failed to get wg listen addr: ", err)
		return err
	}
	common.WgIfaceKeyMap[fmt.Sprintf("%x", md5.Sum([]byte(wgInterface.Device.PublicKey.String())))] = common.RemotePeer{
		PeerKey:   wgInterface.Device.PublicKey.String(),
		Interface: wgInterface.Name,
		Endpoint:  wgListenAddr,
	}
	for _, peerI := range m.Payload.Peers {

		peerConf := m.Payload.PeerMap[peerI.PublicKey.String()]
		if peerI.Endpoint == nil && !(peerConf.IsAttachedExtClient || peerConf.IsExtClient) {
			log.Println("Endpoint nil for peer: ", peerI.PublicKey.String())
			continue
		}
		if peerConf.IsExtClient && !common.IsIngressGateway {
			continue
		}
		shouldProceed := false
		if peerConf.IsExtClient && peerConf.IsAttachedExtClient {
			// check if ext client got endpoint,otherwise continue
			for _, devpeerI := range wgInterface.Device.Peers {
				if devpeerI.PublicKey.String() == peerI.PublicKey.String() && devpeerI.Endpoint != nil {
					peerI.Endpoint = devpeerI.Endpoint
					shouldProceed = true
					break
				}
			}

		} else {
			shouldProceed = true
		}
		if peerConf.IsExtClient && peerConf.IsAttachedExtClient && shouldProceed {
			ctx, cancel := context.WithCancel(context.Background())
			common.ExtClientsWaitTh[peerI.PublicKey.String()] = common.ExtClientPeer{
				Endpoint:   peerI.Endpoint,
				CancelFunc: cancel,
			}
			go proxy.StartSniffer(ctx, wgInterface.Name, m.Payload.WgAddr, peerConf.Address, wgInterface.Port)
		}

		if peerConf.IsExtClient && !peerConf.IsAttachedExtClient {
			peerI.Endpoint = peerConf.IngressGatewayEndPoint
		}
		if shouldProceed {
			common.PeerKeyHashMap[fmt.Sprintf("%x", md5.Sum([]byte(peerI.PublicKey.String())))] = common.RemotePeer{
				Interface:           m.Payload.InterfaceName,
				PeerKey:             peerI.PublicKey.String(),
				IsExtClient:         peerConf.IsExtClient,
				Endpoint:            peerI.Endpoint,
				IsAttachedExtClient: peerConf.IsAttachedExtClient,
			}
		}

		var isRelayed bool
		var relayedTo *net.UDPAddr
		if m.Payload.IsRelayed {
			isRelayed = true
			relayedTo = m.Payload.RelayedTo
		} else {

			isRelayed = peerConf.IsRelayed
			relayedTo = peerConf.RelayedTo

		}
		if !shouldProceed && peerConf.IsAttachedExtClient {
			log.Println("Extclient endpoint not updated yet....skipping")
			// TODO - watch the interface for ext client update
			go func(wgInterface *wg.WGIface, peer *wgtypes.PeerConfig,
				isRelayed bool, relayTo *net.UDPAddr, peerConf PeerConf, ingGwAddr string) {
				addExtClient := false
				ctx, cancel := context.WithCancel(context.Background())
				common.ExtClientsWaitTh[peerI.PublicKey.String()] = common.ExtClientPeer{
					Endpoint:   peer.Endpoint,
					CancelFunc: cancel,
				}
				defer func() {
					if addExtClient {
						log.Println("GOT ENDPOINT for Extclient adding peer...")
						go proxy.StartSniffer(ctx, wgInterface.Name, ingGwAddr, peerConf.Address, wgInterface.Port)
						common.PeerKeyHashMap[fmt.Sprintf("%x", md5.Sum([]byte(peer.PublicKey.String())))] = common.RemotePeer{
							Interface:           wgInterface.Name,
							PeerKey:             peer.PublicKey.String(),
							IsExtClient:         peerConf.IsExtClient,
							IsAttachedExtClient: peerConf.IsAttachedExtClient,
							Endpoint:            peer.Endpoint,
						}

						peerpkg.AddNewPeer(wgInterface, peer, peerConf.Address, isRelayed,
							peerConf.IsExtClient, peerConf.IsAttachedExtClient, relayedTo)
					}
				}()
				for {
					select {
					case <-ctx.Done():
						log.Println("Exiting extclient watch Thread for: ", wgInterface.Device.PublicKey.String())
						return
					default:
						wgInterface, err := wg.NewWGIFace(m.Payload.InterfaceName, "127.0.0.1/32", wg.DefaultMTU)
						if err != nil {
							log.Println("Failed init new interface: ", err)
							return
						}
						for _, devpeerI := range wgInterface.Device.Peers {
							if devpeerI.PublicKey.String() == peer.PublicKey.String() && devpeerI.Endpoint != nil {
								peer.Endpoint = devpeerI.Endpoint
								addExtClient = true
								return
							}
						}
						time.Sleep(time.Second * 5)
					}

				}

			}(wgInterface, &peerI, isRelayed, relayedTo, peerConf, m.Payload.WgAddr)
			continue
		}

		peerpkg.AddNewPeer(wgInterface, &peerI, peerConf.Address, isRelayed,
			peerConf.IsExtClient, peerConf.IsAttachedExtClient, relayedTo)
	}
	log.Printf("------> PEERHASHMAP: %+v\n", common.PeerKeyHashMap)
	log.Printf("-------> WgKeyHashMap: %+v\n", common.WgIfaceKeyMap)
	log.Printf("-------> WgIFaceMap: %+v\n", common.WgIFaceMap)
	return nil
}
