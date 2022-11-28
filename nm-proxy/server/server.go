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

// Proxy.Listen - begins listening for packets
func (p *ProxyServer) Listen(ctx context.Context) {

	// Buffer with indicated body size
	buffer := make([]byte, 65032)
	for {

		select {
		case <-ctx.Done():
			log.Println("--------->### Shutting down Proxy.....")
			// clean up proxy connections
			for iface, ifaceConf := range common.WgIFaceMap {
				log.Println("########------------>  CLEANING UP: ", iface)
				for _, peerI := range ifaceConf.PeerMap {
					peerI.StopConn()
				}
			}
			// close server connection
			NmProxyServer.Server.Close()
			return
		default:
			// Read Packet

			n, source, err := p.Server.ReadFromUDP(buffer)
			if err != nil { // in future log errors?
				log.Println("RECV ERROR: ", err)
				continue
			}
			//go func(buffer []byte, source *net.UDPAddr, n int) {
			origBufferLen := n
			var srcPeerKeyHash, dstPeerKeyHash string
			n, srcPeerKeyHash, dstPeerKeyHash = packet.ExtractInfo(buffer, n)
			//log.Printf("--------> RECV PKT , [SRCKEYHASH: %s], SourceIP: [%s] \n", srcPeerKeyHash, source.IP.String())

			if _, ok := common.WgIfaceKeyMap[dstPeerKeyHash]; !ok {
				// if common.IsIngressGateway {
				// 	log.Println("----> fowarding PKT to EXT client...")
				// 	if val, ok := common.PeerKeyHashMap[dstPeerKeyHash]; ok && val.IsAttachedExtClient {

				// 		log.Printf("-------->Forwarding the pkt to extClient  [ SourceIP: %s ], [ SourceKeyHash: %s ], [ DstIP: %s ], [ DstHashKey: %s ] \n",
				// 			source.String(), srcPeerKeyHash, val.Endpoint.String(), dstPeerKeyHash)
				// 		_, err = NmProxyServer.Server.WriteToUDP(buffer[:n], val.Endpoint)
				// 		if err != nil {
				// 			log.Println("Failed to send to remote: ", err)
				// 		}
				// 		continue

				// 	}
				// }

				if common.IsRelay {

					log.Println("----------> Relaying######")
					// check for routing map and forward to right proxy
					if remoteMap, ok := common.RelayPeerMap[srcPeerKeyHash]; ok {
						if conf, ok := remoteMap[dstPeerKeyHash]; ok {
							log.Printf("--------> Relaying PKT [ SourceIP: %s:%d ], [ SourceKeyHash: %s ], [ DstIP: %s:%d ], [ DstHashKey: %s ] \n",
								source.IP.String(), source.Port, srcPeerKeyHash, conf.Endpoint.String(), conf.Endpoint.Port, dstPeerKeyHash)
							_, err = NmProxyServer.Server.WriteToUDP(buffer[:n+32], conf.Endpoint)
							if err != nil {
								log.Println("Failed to send to remote: ", err)
							}
							//continue
						}
					} else {
						if remoteMap, ok := common.RelayPeerMap[dstPeerKeyHash]; ok {
							if conf, ok := remoteMap[dstPeerKeyHash]; ok {
								log.Printf("--------> Relaying BACK TO RELAYED NODE PKT [ SourceIP: %s ], [ SourceKeyHash: %s ], [ DstIP: %s ], [ DstHashKey: %s ] \n",
									source.String(), srcPeerKeyHash, conf.Endpoint.String(), dstPeerKeyHash)
								_, err = NmProxyServer.Server.WriteToUDP(buffer[:n+32], conf.Endpoint)
								if err != nil {
									log.Println("Failed to send to remote: ", err)
								}
								//continue
							}
						}

					}

				}

			}

			if peerInfo, ok := common.PeerKeyHashMap[srcPeerKeyHash]; ok {
				if ifaceConf, ok := common.WgIFaceMap[peerInfo.Interface]; ok {
					if peerI, ok := ifaceConf.PeerMap[peerInfo.PeerKey]; ok {
						metrics.MetricsMapLock.Lock()
						metric := metrics.MetricsMap[peerInfo.PeerKey]
						metric.TrafficRecieved += uint64(n)
						metric.ConnectionStatus = true
						metrics.MetricsMap[peerInfo.PeerKey] = metric
						metrics.MetricsMapLock.Unlock()
						log.Printf("PROXING TO LOCAL!!!---> %s <<<< %s <<<<<<<< %s   [[ RECV PKT [SRCKEYHASH: %s], [DSTKEYHASH: %s], SourceIP: [%s] ]]\n",
							peerI.LocalConn.RemoteAddr(), peerI.LocalConn.LocalAddr(),
							fmt.Sprintf("%s:%d", source.IP.String(), source.Port), srcPeerKeyHash, dstPeerKeyHash, source.IP.String())
						_, err = peerI.LocalConn.Write(buffer[:n])
						if err != nil {
							log.Println("Failed to proxy to Wg local interface: ", err)
							//continue
						}
						continue

					}
				}

			}
			if peerInfo, ok := common.ExtSourceIpMap[source.String()]; ok {
				if ifaceConf, ok := common.WgIFaceMap[peerInfo.Interface]; ok {
					if peerI, ok := ifaceConf.PeerMap[peerInfo.PeerKey]; ok {
						metrics.MetricsMapLock.Lock()
						metric := metrics.MetricsMap[peerInfo.PeerKey]
						metric.TrafficRecieved += uint64(n)
						metric.ConnectionStatus = true
						metrics.MetricsMap[peerInfo.PeerKey] = metric
						metrics.MetricsMapLock.Unlock()
						log.Printf("PROXING TO LOCAL!!!---> %s <<<< %s <<<<<<<< %s   [[ RECV PKT [SRCKEYHASH: %s], [DSTKEYHASH: %s], SourceIP: [%s] ]]\n",
							peerI.LocalConn.RemoteAddr(), peerI.LocalConn.LocalAddr(),
							fmt.Sprintf("%s:%d", source.IP.String(), source.Port), srcPeerKeyHash, dstPeerKeyHash, source.IP.String())
						_, err = peerI.LocalConn.Write(buffer[:origBufferLen])
						if err != nil {
							log.Println("Failed to proxy to Wg local interface: ", err)
							//continue
						}
						continue

					}
				}
			}
			// unknown peer to proxy -> check if extclient and handle it
			// consume handshake message for ext clients
			msgType := binary.LittleEndian.Uint32(buffer[:4])
			switch msgType {
			case packet.MessageMetricsType:
				metricMsg, err := packet.ConsumeMetricPacket(buffer[:origBufferLen])
				// calc latency
				if err == nil {
					latency := time.Now().UnixMilli() - metricMsg.TimeStamp
					metrics.MetricsMapLock.Lock()
					metric := metrics.MetricsMap[metricMsg.Reciever.PublicKey().String()]
					metric.LastRecordedLatency = latency
					metric.ConnectionStatus = true
					metric.TrafficRecieved += uint64(origBufferLen)
					metrics.MetricsMap[metricMsg.Reciever.PublicKey().String()] = metric
					metrics.MetricsMapLock.Unlock()
				}

			case packet.MessageInitiationType:

				devPriv, devPubkey, err := packet.GetDeviceKeys(common.InterfaceName)
				if err == nil {
					err := packet.ConsumeHandshakeInitiationMsg(false, buffer[:origBufferLen], source, devPubkey, devPriv)
					if err != nil {
						log.Println("---------> @@@ failed to decode HS: ", err)
					}
				} else {
					log.Println("failed to get device keys: ", err)
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
