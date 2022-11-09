package manager

import (
	"crypto/md5"
	"errors"
	"fmt"
	"log"
	"net"
	"runtime"

	"github.com/gravitl/netmaker/nm-proxy/common"
	peerpkg "github.com/gravitl/netmaker/nm-proxy/peer"
	"github.com/gravitl/netmaker/nm-proxy/wg"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type ProxyAction string

type ManagerPayload struct {
	InterfaceName   string                 `json:"interface_name"`
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
	IsAttachedExtClient    bool         `json:"is_attached_ext_client"`
	IngressGatewayEndPoint *net.UDPAddr `json:"ingress_gateway_endpoint"`
	IsRelayed              bool         `json:"is_relayed"`
	RelayedTo              *net.UDPAddr `json:"relayed_to"`
}

const (
	AddInterface ProxyAction = "ADD_INTERFACE"
	DeletePeer   ProxyAction = "DELETE_PEER"
	UpdatePeer   ProxyAction = "UPDATE_PEER"
	RelayPeers   ProxyAction = "RELAY_PEERS"
	RelayUpdate  ProxyAction = "RELAY_UPDATE"
	RelayTo      ProxyAction = "RELAY_TO"
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
				common.IsRelay = mI.Payload.IsRelay
				if mI.Payload.IsRelay {
					mI.RelayPeers()
				}
				mI.ExtClients()
				err := mI.AddInterfaceToProxy()
				if err != nil {
					log.Printf("failed to add interface: [%s] to proxy: %v\n  ", mI.Payload.InterfaceName, err)
				}
			case UpdatePeer:
				//mI.UpdatePeerProxy()
			case DeletePeer:
				mI.DeletePeers()
			case RelayPeers:
				mI.RelayPeers()
			case RelayUpdate:
				mI.RelayUpdate()
			}

		}
	}
}

func (m *ManagerAction) RelayUpdate() {
	common.IsRelay = m.Payload.IsRelay
}

func (m *ManagerAction) ExtClients() {
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
			peer.Endpoint.Port = common.NmProxyPort
			remotePeerKeyHash := fmt.Sprintf("%x", md5.Sum([]byte(peer.PublicKey.String())))
			common.RelayPeerMap[relayedNodePubKeyHash][remotePeerKeyHash] = common.RemotePeer{
				Endpoint: peer.Endpoint,
			}
		}
		relayedNodeConf.RelayedPeerEndpoint.Port = common.NmProxyPort
		common.RelayPeerMap[relayedNodePubKeyHash][relayedNodePubKeyHash] = common.RemotePeer{
			Endpoint: relayedNodeConf.RelayedPeerEndpoint,
		}

	}
}

func (m *ManagerAction) DeletePeers() {
	if len(m.Payload.Peers) == 0 {
		log.Println("No Peers to delete...")
		return
	}
	peersMap, ok := common.WgIFaceMap[m.Payload.InterfaceName]
	if !ok {
		log.Println("interface not found: ", m.Payload.InterfaceName)
		return
	}

	for _, peerI := range m.Payload.Peers {
		if peerConf, ok := peersMap[peerI.PublicKey.String()]; ok {
			peerConf.Proxy.Cancel()
			delete(peersMap, peerI.PublicKey.String())
		}
	}
	common.WgIFaceMap[m.Payload.InterfaceName] = peersMap
}

func (m *ManagerAction) UpdatePeerProxy() {
	if len(m.Payload.Peers) == 0 {
		log.Println("No Peers to add...")
		return
	}
	peers, ok := common.WgIFaceMap[m.Payload.InterfaceName]
	if !ok {
		log.Println("interface not found: ", m.Payload.InterfaceName)
		return
	}

	for _, peerI := range m.Payload.Peers {
		peerConf := m.Payload.PeerMap[peerI.PublicKey.String()]
		if peerI.Endpoint == nil && !peerConf.IsExtClient {
			log.Println("Endpoint nil for peer: ", peerI.PublicKey.String())
			continue
		}

		if peerConf, ok := peers[peerI.PublicKey.String()]; ok {

			peerConf.Config.RemoteWgPort = peerI.Endpoint.Port
			peers[peerI.PublicKey.String()] = peerConf
			common.WgIFaceMap[m.Payload.InterfaceName] = peers
			log.Printf("---->####### Updated PEER: %+v\n", peerConf)
		}
	}

}

func cleanUp(iface string) {
	if peers, ok := common.WgIFaceMap[iface]; ok {
		log.Println("########------------>  CLEANING UP: ", iface)
		for _, peerI := range peers {
			peerI.Proxy.Cancel()
		}
	}
	delete(common.WgIFaceMap, iface)
}

func (m *ManagerAction) AddInterfaceToProxy() error {
	var err error
	if m.Payload.InterfaceName == "" {
		return errors.New("interface cannot be empty")
	}
	if len(m.Payload.Peers) == 0 {
		log.Println("No Peers to add...")
		return nil
	}
	ifaceName := m.Payload.InterfaceName
	log.Println("--------> IFACE: ", ifaceName)
	if runtime.GOOS == "darwin" {
		ifaceName, err = wg.GetRealIface(ifaceName)
		if err != nil {
			log.Println("failed to get real iface: ", err)
		}
	}
	cleanUp(ifaceName)

	wgInterface, err := wg.NewWGIFace(ifaceName, "127.0.0.1/32", wg.DefaultMTU)
	if err != nil {
		log.Println("Failed init new interface: ", err)
		return err
	}
	log.Printf("wg: %+v\n", wgInterface)

	for _, peerI := range m.Payload.Peers {
		peerConf := m.Payload.PeerMap[peerI.PublicKey.String()]
		if peerI.Endpoint == nil && !peerConf.IsExtClient {
			log.Println("Endpoint nil for peer: ", peerI.PublicKey.String())
			continue
		}
		common.PeerKeyHashMap[fmt.Sprintf("%x", md5.Sum([]byte(peerI.PublicKey.String())))] = common.RemotePeer{
			Interface: ifaceName,
			PeerKey:   peerI.PublicKey.String(),
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

		peerpkg.AddNewPeer(wgInterface, &peerI, isRelayed,
			peerConf.IsExtClient, peerConf.IngressGatewayEndPoint, relayedTo)
	}
	log.Printf("------> PEERHASHMAP: %+v\n", common.PeerKeyHashMap)
	return nil
}
