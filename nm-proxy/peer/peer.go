package peer

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
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

func GetNodeInfo(cfg *config.ClientConfig) (models.NodeGet, error) {
	var nodeGET models.NodeGet
	token, err := common.Authenticate(cfg)
	if err != nil {
		return nodeGET, err
	}
	url := fmt.Sprintf("https://%s/api/nodes/%s/%s", cfg.Server.API, cfg.Network, cfg.Node.ID)
	response, err := common.API("", http.MethodGet, url, token)
	if err != nil {
		return nodeGET, err
	}
	if response.StatusCode != http.StatusOK {
		bytes, err := io.ReadAll(response.Body)
		if err != nil {
			fmt.Println(err)
		}
		return nodeGET, (fmt.Errorf("%s %w", string(bytes), err))
	}
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(&nodeGET); err != nil {
		return nodeGET, fmt.Errorf("error decoding node %w", err)
	}
	return nodeGET, nil
}

func AddNewPeer(pserver *server.ProxyServer, wgInterface *wg.WGIface, peer *wgtypes.PeerConfig) error {

	c := proxy.Config{
		Port:         peer.Endpoint.Port,
		WgListenAddr: "127.0.0.1",
		RemoteKey:    peer.PublicKey.String(),
		WgInterface:  wgInterface,
		AllowedIps:   peer.AllowedIPs,
		ProxyServer:  pserver,
	}
	p := proxy.NewProxy(c)
	remoteConn, err := net.Dial("udp", fmt.Sprintf("%s:%d", peer.Endpoint.IP.String(), common.NmProxyPort))
	if err != nil {
		return err
	}
	log.Printf("Starting proxy for Peer: %s\n", peer.PublicKey.String())
	err = p.Start(remoteConn)
	if err != nil {
		return err
	}
	log.Println("-------> Here1")
	connConf := common.ConnConfig{
		Key:      peer.PublicKey.String(),
		LocalKey: "",
		ProxyConfig: common.Config{
			Port:         peer.Endpoint.Port,
			WgListenAddr: "127.0.0.1",
			RemoteKey:    peer.PublicKey.String(),
			WgInterface:  wgInterface,
			AllowedIps:   peer.AllowedIPs,
			//ProxyServer:  pserver,
		},
		AllowedIPs:      peer.AllowedIPs,
		RemoteProxyIP:   net.ParseIP(peer.Endpoint.IP.String()),
		RemoteWgPort:    peer.Endpoint.Port,
		RemoteProxyPort: common.NmProxyPort,
	}
	peerProxy := common.Proxy{
		Ctx:        p.Ctx,
		Cancel:     p.Cancel,
		Config:     connConf.ProxyConfig,
		RemoteConn: remoteConn,
		LocalConn:  p.LocalConn,
	}
	peerConn := common.Conn{
		Config: connConf,
		Proxy:  peerProxy,
	}
	common.Peers[peer.PublicKey.String()] = &peerConn
	return nil
}
