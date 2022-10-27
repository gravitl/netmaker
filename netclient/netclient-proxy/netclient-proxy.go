package netclientproxy

import (
	"fmt"
	"log"
	"net"
	"runtime"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/netclient-proxy/common"
	peerpkg "github.com/gravitl/netmaker/netclient/netclient-proxy/peer"
	"github.com/gravitl/netmaker/netclient/netclient-proxy/server"
	"github.com/gravitl/netmaker/netclient/netclient-proxy/stun"
	"github.com/gravitl/netmaker/netclient/netclient-proxy/wg"
	"github.com/gravitl/netmaker/netclient/wireguard"
)

func Start() {
	log.Println("Starting Proxy...")
	hInfo := stun.GetHostInfo()
	stun.Host = hInfo
	log.Printf("HOSTINFO: %+v", hInfo)
	// start the netclient proxy server
	p, err := server.CreateProxyServer(0, 0, hInfo.PrivIp.String())
	if err != nil {
		log.Fatal("failed to create proxy: ", err)
	}
	go p.Listen()
	networks, _ := ncutils.GetSystemNetworks()
	for _, network := range networks {
		logger.Log(3, "initializing network", network)
		cfg := config.ClientConfig{}
		cfg.Network = network
		cfg.ReadConfig()
		node, err := peerpkg.GetNodeInfo(&cfg)
		if err != nil {
			log.Println("Failed to get node info: ", err)
			continue
		}
		for _, peerI := range node.Peers {
			ifaceName := node.Node.Interface
			log.Println("--------> IFACE: ", ifaceName)
			if runtime.GOOS == "darwin" {
				ifaceName, err = wireguard.GetRealIface(ifaceName)
				if err != nil {
					log.Println("failed to get real iface: ", err)
				}
			}
			wgInterface, err := wg.NewWGIFace(ifaceName, "127.0.0.1/32", wg.DefaultMTU)
			if err != nil {
				log.Fatal("Failed init new interface: ", err)
			}
			log.Printf("wg: %+v\n", wgInterface)
			peerpkg.AddNewPeer(p, wgInterface, &peerI)
			if val, ok := common.RemoteEndpointsMap[peerI.Endpoint.IP.String()]; ok {
				val = append(val, peerI.PublicKey.String())
				common.RemoteEndpointsMap[peerI.Endpoint.IP.String()] = val
			} else {
				common.RemoteEndpointsMap[peerI.Endpoint.IP.String()] = []string{peerI.PublicKey.String()}
			}

		}

	}
	fmt.Printf("\nPEERS-------> %+v\n", common.Peers)
	fmt.Printf("\nREMOTE ENDPOINTS-------> %+v\n", common.RemoteEndpointsMap)
	select {}
}

// IsPublicIP indicates whether IP is public or not.
func IsPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() {
		return false
	}
	return true
}
