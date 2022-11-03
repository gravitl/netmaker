package server

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/gravitl/netmaker/nm-proxy/common"
	"github.com/gravitl/netmaker/nm-proxy/packet"
)

var (
	NmProxyServer = &ProxyServer{}
)

const (
	defaultBodySize = 10000
	defaultPort     = common.NmProxyPort
)

type Config struct {
	Port     int
	BodySize int
	Addr     net.Addr
}

type ProxyServer struct {
	Config Config
	Server *net.UDPConn
}

// Proxy.Listen - begins listening for packets
func (p *ProxyServer) Listen() {

	// Buffer with indicated body size
	buffer := make([]byte, 1502)
	for {
		// Read Packet
		n, source, err := p.Server.ReadFromUDP(buffer)
		if err != nil { // in future log errors?
			log.Println("RECV ERROR: ", err)
			continue
		}
		var localWgPort int
		var srcPeerKeyHash string
		localWgPort, n, srcPeerKeyHash, err = packet.ExtractInfo(buffer, n)
		if err != nil {
			log.Println("failed to extract info: ", err)
			continue
		}
		// log.Printf("--------> RECV PKT [DSTPORT: %d], [SRCKEYHASH: %s], SourceIP: [%s] \n", localWgPort, srcPeerKeyHash, source.IP.String())
		if peerInfo, ok := common.PeerKeyHashMap[srcPeerKeyHash]; ok {
			if peers, ok := common.WgIFaceMap[peerInfo.Interface]; ok {
				if peerI, ok := peers[peerInfo.PeerKey]; ok {
					// if peerI.Config.LocalWgPort == int(localWgPort) {
					log.Printf("PROXING TO LOCAL!!!---> %s <<<< %s <<<<<<<< %s   [[ RECV PKT [DSTPORT: %d], [SRCKEYHASH: %s], SourceIP: [%s] ]]\n",
						peerI.Proxy.LocalConn.RemoteAddr(), peerI.Proxy.LocalConn.LocalAddr(),
						fmt.Sprintf("%s:%d", source.IP.String(), source.Port), localWgPort, srcPeerKeyHash, source.IP.String())
					_, err = peerI.Proxy.LocalConn.Write(buffer[:n])
					if err != nil {
						log.Println("Failed to proxy to Wg local interface: ", err)
						continue
					}

					// }
				}
			}

		}

	}
}

// Create - creats a proxy listener
// port - port for proxy to listen on localhost
// bodySize - default 10000, leave 0 to use default
// addr - the address for proxy to listen on
// forwards - indicate address to forward to, {"<address:port>",...} format
func (p *ProxyServer) CreateProxyServer(port, bodySize int, addr string) (err error) {
	if p == nil {
		p = &ProxyServer{}
	}
	p.Config.Port = port
	p.Config.BodySize = bodySize
	p.setDefaults()
	p.Server, err = net.ListenUDP("udp", &net.UDPAddr{
		Port: p.Config.Port,
		IP:   net.ParseIP(addr),
	})
	return
}

func (p *ProxyServer) KeepAlive(ip string, port int) {
	for {
		_, _ = p.Server.Write([]byte("hello-proxy"))
		//fmt.Println("Sending MSg: ", err)
		time.Sleep(time.Second)
	}
}

// Proxy.setDefaults - sets all defaults of proxy listener
func (p *ProxyServer) setDefaults() {
	p.setDefaultBodySize()
	p.setDefaultPort()
}

// Proxy.setDefaultPort - sets default port of Proxy listener if 0
func (p *ProxyServer) setDefaultPort() {
	if p.Config.Port == 0 {
		p.Config.Port = defaultPort
	}
}

// Proxy.setDefaultBodySize - sets default body size of Proxy listener if 0
func (p *ProxyServer) setDefaultBodySize() {
	if p.Config.BodySize == 0 {
		p.Config.BodySize = defaultBodySize
	}
}
