package manager

import (
	"errors"
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
				mI.AddInterfaceToProxy()
			}

		}
	}
}
func cleanUp(iface string) {
	if peers, ok := common.WgIFaceMap[iface]; ok {
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

		peerpkg.AddNewPeer(wgInterface, &peerI)
		if val, ok := common.RemoteEndpointsMap[peerI.Endpoint.IP.String()]; ok {

			val = append(val, common.RemotePeer{
				Interface: ifaceName,
				PeerKey:   peerI.PublicKey.String(),
			})
			common.RemoteEndpointsMap[peerI.Endpoint.IP.String()] = val
		} else {
			common.RemoteEndpointsMap[peerI.Endpoint.IP.String()] = []common.RemotePeer{{
				Interface: ifaceName,
				PeerKey:   peerI.PublicKey.String(),
			}}
		}

	}
	return nil
}
