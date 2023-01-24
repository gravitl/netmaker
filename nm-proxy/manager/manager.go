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

	"github.com/gravitl/netmaker/nm-proxy/common"
	"github.com/gravitl/netmaker/nm-proxy/models"
	peerpkg "github.com/gravitl/netmaker/nm-proxy/peer"
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
	if common.WgIfaceMap.Iface.Name == m.Payload.InterfaceName {
		cleanUpInterface()
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
			common.RelayPeerMap[relayedNodePubKeyHash] = make(map[string]models.RemotePeer)
		}
		for _, peer := range relayedNodeConf.Peers {
			if peer.Endpoint != nil {
				peer.Endpoint.Port = models.NmProxyPort
				remotePeerKeyHash := fmt.Sprintf("%x", md5.Sum([]byte(peer.PublicKey.String())))
				common.RelayPeerMap[relayedNodePubKeyHash][remotePeerKeyHash] = models.RemotePeer{
					Endpoint: peer.Endpoint,
				}
			}

		}
		relayedNodeConf.RelayedPeerEndpoint.Port = models.NmProxyPort
		common.RelayPeerMap[relayedNodePubKeyHash][relayedNodePubKeyHash] = models.RemotePeer{
			Endpoint: relayedNodeConf.RelayedPeerEndpoint,
		}

	}
}

func cleanUpInterface() {
	log.Println("########------------>  CLEANING UP: ", common.WgIfaceMap.Iface.Name)
	for _, peerI := range common.WgIfaceMap.PeerMap {
		peerI.Mutex.Lock()
		peerI.StopConn()
		peerI.Mutex.Unlock()
		delete(common.WgIfaceMap.PeerMap, peerI.Key.String())
	}
	common.WgIfaceMap.PeerMap = make(map[string]*models.Conn)
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
	common.InterfaceName = m.Payload.InterfaceName
	wgIface, err = wg.NewWGIFace(m.Payload.InterfaceName, "127.0.0.1/32", wg.DefaultMTU)
	if err != nil {
		log.Println("Failed init new interface: ", err)
		return nil, err
	}

	if common.WgIfaceMap.Iface == nil {
		for i := len(m.Payload.Peers) - 1; i >= 0; i-- {
			if !m.Payload.PeerMap[m.Payload.Peers[i].PublicKey.String()].Proxy {
				log.Println("-----------> skipping peer, proxy is off: ", m.Payload.Peers[i].PublicKey)
				if err := wgIface.Update(m.Payload.Peers[i], false); err != nil {
					log.Println("falied to update peer: ", err)
				}
				m.Payload.Peers = append(m.Payload.Peers[:i], m.Payload.Peers[i+1:]...)
				continue
			}
		}
		common.WgIfaceMap.Iface = wgIface.Device
		common.WgIfaceMap.IfaceKeyHash = fmt.Sprintf("%x", md5.Sum([]byte(wgIface.Device.PublicKey.String())))
		return wgIface, nil
	}
	wgProxyConf := common.WgIfaceMap
	if m.Payload.IsRelay {
		m.RelayPeers()
	}
	common.IsRelay = m.Payload.IsRelay
	// check if node is getting relayed
	if common.IsRelayed != m.Payload.IsRelayed {
		common.IsRelayed = m.Payload.IsRelayed
		cleanUpInterface()
		return wgIface, nil
	}

	// sync map with wg device config
	// check if listen port has changed
	if wgIface.Device.ListenPort != wgProxyConf.Iface.ListenPort {
		// reset proxy for this interface
		cleanUpInterface()
		return wgIface, nil
	}
	// check device conf different from proxy
	wgProxyConf.Iface = wgIface.Device
	// sync peer map with new update
	for _, currPeerI := range wgProxyConf.Iface.Peers {
		if _, ok := m.Payload.PeerMap[currPeerI.PublicKey.String()]; !ok {
			if val, ok := wgProxyConf.PeerMap[currPeerI.PublicKey.String()]; ok {
				val.Mutex.Lock()
				if val.IsAttachedExtClient {
					log.Println("------> Deleting ExtClient Watch Thread: ", currPeerI.PublicKey.String())
					if val, ok := common.ExtClientsWaitTh[currPeerI.PublicKey.String()]; ok {
						val.CancelFunc()
						delete(common.ExtClientsWaitTh, currPeerI.PublicKey.String())
					}
					log.Println("-----> Deleting Ext Client from Src Ip Map: ", currPeerI.PublicKey.String())
					delete(common.ExtSourceIpMap, val.Config.PeerConf.Endpoint.String())
				}
				val.StopConn()
				val.Mutex.Unlock()
				delete(wgProxyConf.PeerMap, currPeerI.PublicKey.String())
			}

			// delete peer from interface
			log.Println("CurrPeer Not Found, Deleting Peer from Interface: ", currPeerI.PublicKey.String())
			if err := wgIface.RemovePeer(currPeerI.PublicKey.String()); err != nil {
				log.Println("failed to remove peer: ", currPeerI.PublicKey.String(), err)
			}

			delete(common.PeerKeyHashMap, fmt.Sprintf("%x", md5.Sum([]byte(currPeerI.PublicKey.String()))))

		}
	}
	for i := len(m.Payload.Peers) - 1; i >= 0; i-- {

		if currentPeer, ok := wgProxyConf.PeerMap[m.Payload.Peers[i].PublicKey.String()]; ok {
			currentPeer.Mutex.Lock()
			if currentPeer.IsAttachedExtClient {
				m.Payload.Peers = append(m.Payload.Peers[:i], m.Payload.Peers[i+1:]...)
				continue
			}
			// check if proxy is off for the peer
			if !m.Payload.PeerMap[m.Payload.Peers[i].PublicKey.String()].Proxy {

				// cleanup proxy connections for the peer
				currentPeer.StopConn()
				delete(wgProxyConf.PeerMap, currentPeer.Key.String())
				// update the peer with actual endpoint
				if err := wgIface.Update(m.Payload.Peers[i], false); err != nil {
					log.Println("falied to update peer: ", err)
				}
				m.Payload.Peers = append(m.Payload.Peers[:i], m.Payload.Peers[i+1:]...)
				continue

			}
			// check if peer is not connected to proxy
			devPeer, err := wg.GetPeer(m.Payload.InterfaceName, currentPeer.Key.String())
			if err == nil {
				log.Printf("---------> COMAPRING ENDPOINT: DEV: %s, Proxy: %s", devPeer.Endpoint.String(), currentPeer.Config.LocalConnAddr.String())
				if devPeer.Endpoint.String() != currentPeer.Config.LocalConnAddr.String() {
					log.Println("---------> endpoint is not set to proxy: ", currentPeer.Key)
					currentPeer.StopConn()
					delete(wgProxyConf.PeerMap, currentPeer.Key.String())
					continue
				}
			}
			//check if peer is being relayed
			if currentPeer.IsRelayed != m.Payload.PeerMap[m.Payload.Peers[i].PublicKey.String()].IsRelayed {
				log.Println("---------> peer relay status has been changed: ", currentPeer.Key)
				currentPeer.StopConn()
				delete(wgProxyConf.PeerMap, currentPeer.Key.String())
				continue
			}
			// check if relay endpoint has been changed
			if currentPeer.RelayedEndpoint != nil &&
				m.Payload.PeerMap[m.Payload.Peers[i].PublicKey.String()].RelayedTo != nil &&
				currentPeer.RelayedEndpoint.String() != m.Payload.PeerMap[m.Payload.Peers[i].PublicKey.String()].RelayedTo.String() {
				log.Println("---------> peer relay endpoint has been changed: ", currentPeer.Key)
				currentPeer.StopConn()
				delete(wgProxyConf.PeerMap, currentPeer.Key.String())
				continue
			}
			if !reflect.DeepEqual(m.Payload.Peers[i], *currentPeer.Config.PeerConf) {
				if currentPeer.Config.RemoteConnAddr.IP.String() != m.Payload.Peers[i].Endpoint.IP.String() {
					log.Println("----------> Resetting proxy for Peer: ", currentPeer.Key, m.Payload.InterfaceName)
					currentPeer.StopConn()
					currentPeer.Mutex.Unlock()
					delete(wgProxyConf.PeerMap, currentPeer.Key.String())
					continue
				} else {

					log.Println("----->##### Updating Peer on Interface: ", m.Payload.InterfaceName, currentPeer.Key)
					updatePeerConf := m.Payload.Peers[i]
					localUdpAddr, err := net.ResolveUDPAddr("udp", currentPeer.Config.LocalConnAddr.String())
					if err == nil {
						updatePeerConf.Endpoint = localUdpAddr
					}
					if err := wgIface.Update(updatePeerConf, true); err != nil {
						log.Println("failed to update peer: ", currentPeer.Key, err)
					}
					currentPeer.Config.PeerConf = &m.Payload.Peers[i]
					wgProxyConf.PeerMap[currentPeer.Key.String()] = currentPeer
					// delete the peer from the list
					log.Println("-----------> deleting peer from list: ", m.Payload.Peers[i].PublicKey)
					m.Payload.Peers = append(m.Payload.Peers[:i], m.Payload.Peers[i+1:]...)

				}

			} else {
				// delete the peer from the list
				log.Println("-----------> No updates observed so deleting peer: ", m.Payload.Peers[i].PublicKey)
				m.Payload.Peers = append(m.Payload.Peers[:i], m.Payload.Peers[i+1:]...)
			}
			currentPeer.Mutex.Unlock()

		} else if !m.Payload.PeerMap[m.Payload.Peers[i].PublicKey.String()].Proxy && !m.Payload.PeerMap[m.Payload.Peers[i].PublicKey.String()].IsAttachedExtClient {
			log.Println("-----------> skipping peer, proxy is off: ", m.Payload.Peers[i].PublicKey)
			if err := wgIface.Update(m.Payload.Peers[i], false); err != nil {
				log.Println("falied to update peer: ", err)
			}
			m.Payload.Peers = append(m.Payload.Peers[:i], m.Payload.Peers[i+1:]...)
		}
	}

	// sync dev peers with new update

	common.WgIfaceMap = wgProxyConf

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
	for _, peerI := range m.Payload.Peers {

		peerConf := m.Payload.PeerMap[peerI.PublicKey.String()]
		if peerI.Endpoint == nil && !(peerConf.IsAttachedExtClient || peerConf.IsExtClient) {
			log.Println("Endpoint nil for peer: ", peerI.PublicKey.String())
			continue
		}

		if peerConf.IsExtClient && !peerConf.IsAttachedExtClient {
			peerI.Endpoint = peerConf.IngressGatewayEndPoint
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
		if peerConf.IsAttachedExtClient {
			log.Println("Extclient Thread...")
			go func(wgInterface *wg.WGIface, peer *wgtypes.PeerConfig,
				isRelayed bool, relayTo *net.UDPAddr, peerConf PeerConf, ingGwAddr string) {
				addExtClient := false
				commChan := make(chan *net.UDPAddr, 100)
				ctx, cancel := context.WithCancel(context.Background())
				common.ExtClientsWaitTh[peerI.PublicKey.String()] = models.ExtClientPeer{
					CancelFunc: cancel,
					CommChan:   commChan,
				}
				defer func() {
					if addExtClient {
						log.Println("GOT ENDPOINT for Extclient adding peer...")

						common.ExtSourceIpMap[peer.Endpoint.String()] = models.RemotePeer{
							Interface:           wgInterface.Name,
							PeerKey:             peer.PublicKey.String(),
							IsExtClient:         peerConf.IsExtClient,
							IsAttachedExtClient: peerConf.IsAttachedExtClient,
							Endpoint:            peer.Endpoint,
						}

						peerpkg.AddNewPeer(wgInterface, peer, peerConf.Address, isRelayed,
							peerConf.IsExtClient, peerConf.IsAttachedExtClient, relayedTo)

					}
					log.Println("Exiting extclient watch Thread for: ", peer.PublicKey.String())
				}()
				for {
					select {
					case <-ctx.Done():
						return
					case endpoint := <-commChan:
						if endpoint != nil {
							addExtClient = true
							peer.Endpoint = endpoint
							delete(common.ExtClientsWaitTh, peer.PublicKey.String())
							return
						}
					}

				}

			}(wgInterface, &peerI, isRelayed, relayedTo, peerConf, m.Payload.WgAddr)
			continue
		}

		peerpkg.AddNewPeer(wgInterface, &peerI, peerConf.Address, isRelayed,
			peerConf.IsExtClient, peerConf.IsAttachedExtClient, relayedTo)

	}
	return nil
}
