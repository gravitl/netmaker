package server

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/gravitl/netmaker/nm-proxy/common"
	"github.com/gravitl/netmaker/nm-proxy/metrics"
	"github.com/gravitl/netmaker/nm-proxy/models"
	"github.com/gravitl/netmaker/nm-proxy/packet"
)

var (
	NmProxyServer = &ProxyServer{}
)

const (
	defaultBodySize = 10000
	defaultPort     = models.NmProxyPort
)

type Config struct {
	Port     int
	BodySize int
	IsRelay  bool
	Addr     net.Addr
}

type ProxyServer struct {
	Config Config
	Server *net.UDPConn
}

func (p *ProxyServer) Close() {
	log.Println("--------->### Shutting down Proxy.....")
	// clean up proxy connections
	for _, peerI := range common.WgIfaceMap.PeerMap {
		peerI.Mutex.Lock()
		peerI.StopConn()
		peerI.Mutex.Unlock()
	}
	// close server connection
	NmProxyServer.Server.Close()
}

// Proxy.Listen - begins listening for packets
func (p *ProxyServer) Listen(ctx context.Context) {

	// Buffer with indicated body size
	buffer := make([]byte, 65036)
	for {

		select {
		case <-ctx.Done():
			p.Close()
			return
		default:
			// Read Packet

			n, source, err := p.Server.ReadFromUDP(buffer)
			if err != nil || source == nil { // in future log errors?
				log.Println("RECV ERROR: ", err)
				continue
			}
			//go func(buffer []byte, source *net.UDPAddr, n int) {
			proxyTransportMsg := true
			var srcPeerKeyHash, dstPeerKeyHash string
			n, srcPeerKeyHash, dstPeerKeyHash, err = packet.ExtractInfo(buffer, n)
			if err != nil {
				log.Println("proxy transport message not found: ", err)
				proxyTransportMsg = false
			}
			if proxyTransportMsg {
				p.proxyIncomingPacket(buffer[:], source, n, srcPeerKeyHash, dstPeerKeyHash)
				continue
			} else {
				// unknown peer to proxy -> check if extclient and handle it
				if handleExtClients(buffer[:], n, source) {
					continue
				}

			}
			handleMsgs(buffer, n, source)

		}
	}
}

func handleMsgs(buffer []byte, n int, source *net.UDPAddr) {

	msgType := binary.LittleEndian.Uint32(buffer[:4])
	switch packet.MessageType(msgType) {
	case packet.MessageMetricsType:
		metricMsg, err := packet.ConsumeMetricPacket(buffer[:n])
		// calc latency
		if err == nil {
			log.Printf("------->$$$$$ Recieved Metric Pkt: %+v, FROM:%s\n", metricMsg, source.String())
			if metricMsg.Sender == common.WgIfaceMap.Iface.PublicKey {
				latency := time.Now().UnixMilli() - metricMsg.TimeStamp
				metrics.MetricsMapLock.Lock()
				metric := metrics.MetricsMap[metricMsg.Reciever.String()]
				metric.LastRecordedLatency = uint64(latency)
				metric.ConnectionStatus = true
				metric.TrafficRecieved += float64(n) / (1 << 20)
				metrics.MetricsMap[metricMsg.Reciever.String()] = metric
				metrics.MetricsMapLock.Unlock()
			} else if metricMsg.Reciever == common.WgIfaceMap.Iface.PublicKey {
				// proxy it back to the sender
				log.Println("------------> $$$ SENDING back the metric pkt to the source: ", source.String())
				_, err = NmProxyServer.Server.WriteToUDP(buffer[:n], source)
				if err != nil {
					log.Println("Failed to send metric packet to remote: ", err)
				}
				metrics.MetricsMapLock.Lock()
				metric := metrics.MetricsMap[metricMsg.Sender.String()]
				metric.ConnectionStatus = true
				metric.TrafficRecieved += float64(n) / (1 << 20)
				metrics.MetricsMap[metricMsg.Sender.String()] = metric
				metrics.MetricsMapLock.Unlock()
			}
		}
	case packet.MessageProxyUpdateType:
		msg, err := packet.ConsumeProxyUpdateMsg(buffer[:n])
		if err == nil {
			switch msg.Action {
			case packet.UpdateListenPort:
				if peer, ok := common.WgIfaceMap.PeerMap[msg.Sender.String()]; ok {
					peer.Mutex.Lock()
					if peer.Config.PeerEndpoint.Port != int(msg.ListenPort) {
						// update peer conn
						peer.Config.PeerEndpoint.Port = int(msg.ListenPort)
						common.WgIfaceMap.PeerMap[msg.Sender.String()] = peer
						log.Println("--------> Resetting Proxy Conn For Peer ", msg.Sender.String())
						peer.Mutex.Unlock()
						peer.ResetConn()
						return
					}
					peer.Mutex.Unlock()

				}
			}
		}
	// consume handshake message for ext clients
	case packet.MessageInitiationType:

		err := packet.ConsumeHandshakeInitiationMsg(false, buffer[:n], source,
			packet.NoisePublicKey(common.WgIfaceMap.Iface.PublicKey), packet.NoisePrivateKey(common.WgIfaceMap.Iface.PrivateKey))
		if err != nil {
			log.Println("---------> @@@ failed to decode HS: ", err)
		}
	}
}

func handleExtClients(buffer []byte, n int, source *net.UDPAddr) bool {
	isExtClient := false
	if peerInfo, ok := common.ExtSourceIpMap[source.String()]; ok {
		if peerI, ok := common.WgIfaceMap.PeerMap[peerInfo.PeerKey]; ok {
			peerI.Mutex.RLock()
			peerI.Config.RecieverChan <- buffer[:n]
			metrics.MetricsMapLock.Lock()
			metric := metrics.MetricsMap[peerInfo.PeerKey]
			metric.TrafficRecieved += float64(n) / (1 << 20)
			metric.ConnectionStatus = true
			metrics.MetricsMap[peerInfo.PeerKey] = metric
			metrics.MetricsMapLock.Unlock()
			peerI.Mutex.RUnlock()
			isExtClient = true
		}

	}
	return isExtClient
}

func (p *ProxyServer) proxyIncomingPacket(buffer []byte, source *net.UDPAddr, n int, srcPeerKeyHash, dstPeerKeyHash string) {
	var err error
	//log.Printf("--------> RECV PKT , [SRCKEYHASH: %s], SourceIP: [%s] \n", srcPeerKeyHash, source.IP.String())

	if common.WgIfaceMap.IfaceKeyHash != dstPeerKeyHash && common.IsRelay {

		log.Println("----------> Relaying######")
		// check for routing map and forward to right proxy
		if remoteMap, ok := common.RelayPeerMap[srcPeerKeyHash]; ok {
			if conf, ok := remoteMap[dstPeerKeyHash]; ok {
				log.Printf("--------> Relaying PKT [ SourceIP: %s:%d ], [ SourceKeyHash: %s ], [ DstIP: %s:%d ], [ DstHashKey: %s ] \n",
					source.IP.String(), source.Port, srcPeerKeyHash, conf.Endpoint.String(), conf.Endpoint.Port, dstPeerKeyHash)
				_, err = p.Server.WriteToUDP(buffer[:n+packet.MessageProxySize], conf.Endpoint)
				if err != nil {
					log.Println("Failed to send to remote: ", err)
				}
				return
			}
		} else {
			if remoteMap, ok := common.RelayPeerMap[dstPeerKeyHash]; ok {
				if conf, ok := remoteMap[dstPeerKeyHash]; ok {
					log.Printf("--------> Relaying BACK TO RELAYED NODE PKT [ SourceIP: %s ], [ SourceKeyHash: %s ], [ DstIP: %s ], [ DstHashKey: %s ] \n",
						source.String(), srcPeerKeyHash, conf.Endpoint.String(), dstPeerKeyHash)
					_, err = p.Server.WriteToUDP(buffer[:n+packet.MessageProxySize], conf.Endpoint)
					if err != nil {
						log.Println("Failed to send to remote: ", err)
					}
					return
				}
			}

		}
	}

	if peerInfo, ok := common.PeerKeyHashMap[srcPeerKeyHash]; ok {

		log.Printf("PROXING TO LOCAL!!!---> %s <<<< %s <<<<<<<< %s   [[ RECV PKT [SRCKEYHASH: %s], [DSTKEYHASH: %s], SourceIP: [%s] ]]\n",
			peerInfo.LocalConn.RemoteAddr(), peerInfo.LocalConn.LocalAddr(),
			fmt.Sprintf("%s:%d", source.IP.String(), source.Port), srcPeerKeyHash, dstPeerKeyHash, source.IP.String())
		_, err = peerInfo.LocalConn.Write(buffer[:n])
		if err != nil {
			log.Println("Failed to proxy to Wg local interface: ", err)
			//continue
		}

		go func(n int, peerKey string) {
			metrics.MetricsMapLock.Lock()
			metric := metrics.MetricsMap[peerKey]
			metric.TrafficRecieved += float64(n) / (1 << 20)
			metric.ConnectionStatus = true
			metrics.MetricsMap[peerKey] = metric
			metrics.MetricsMapLock.Unlock()
		}(n, peerInfo.PeerKey)
		return

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
		_, _ = p.Server.WriteToUDP([]byte("hello-proxy"), &net.UDPAddr{
			IP:   net.ParseIP(ip),
			Port: port,
		})
		//log.Println("Sending MSg: ", ip, port, err)
		time.Sleep(time.Second * 5)
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
