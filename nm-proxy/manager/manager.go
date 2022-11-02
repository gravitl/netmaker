package manager

import (
	"crypto/md5"
	"errors"
	"fmt"
	"log"
	"runtime"

	"github.com/gravitl/netmaker/netclient/wireguard"
	"github.com/gravitl/netmaker/nm-proxy/common"
	peerpkg "github.com/gravitl/netmaker/nm-proxy/peer"
	"github.com/gravitl/netmaker/nm-proxy/wg"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type ProxyAction string

type ManagerPayload struct {
	InterfaceName string
	Peers         []wgtypes.PeerConfig
}

const (
	AddInterface ProxyAction = "ADD_INTERFACE"
	DeletePeer   ProxyAction = "DELETE_PEER"
	UpdatePeer   ProxyAction = "UPDATE_PEER"
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
				err := mI.AddInterfaceToProxy()
				if err != nil {
					log.Printf("failed to add interface: [%s] to proxy: %v\n  ", mI.Payload.InterfaceName, err)
				}
			case UpdatePeer:
				mI.UpdatePeerProxy()
			case DeletePeer:

			}

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
		if peerI.Endpoint == nil {
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
		ifaceName, err = wireguard.GetRealIface(ifaceName)
		if err != nil {
			log.Println("failed to get real iface: ", err)
		}
	}
	cleanUp(ifaceName)

	wgInterface, err := wg.NewWGIFace(ifaceName, "127.0.0.1/32", wg.DefaultMTU)
	if err != nil {
		log.Fatal("Failed init new interface: ", err)
	}
	log.Printf("wg: %+v\n", wgInterface)

	for _, peerI := range m.Payload.Peers {
		if peerI.Endpoint == nil {
			log.Println("Endpoint nil for peer: ", peerI.PublicKey.String())
			continue
		}
		common.PeerKeyHashMap[fmt.Sprintf("%x", md5.Sum([]byte(peerI.PublicKey.String())))] = common.RemotePeer{
			Interface: ifaceName,
			PeerKey:   peerI.PublicKey.String(),
		}
		peerpkg.AddNewPeer(wgInterface, &peerI)
	}
	log.Printf("------> PEERHASHMAP: %+v\n", common.PeerKeyHashMap)
	return nil
}
