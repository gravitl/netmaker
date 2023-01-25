package proxy

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/c-robinson/iplib"
	"github.com/google/uuid"
	"github.com/gravitl/netmaker/nm-proxy/common"
	"github.com/gravitl/netmaker/nm-proxy/metrics"
	"github.com/gravitl/netmaker/nm-proxy/models"
	"github.com/gravitl/netmaker/nm-proxy/packet"
	"github.com/gravitl/netmaker/nm-proxy/server"
	"github.com/gravitl/netmaker/nm-proxy/stun"
	"github.com/gravitl/netmaker/nm-proxy/wg"
)

func NewProxy(config models.ProxyConfig) *Proxy {
	p := &Proxy{Config: config}
	p.Ctx, p.Cancel = context.WithCancel(context.Background())
	return p
}

func (p *Proxy) proxyToRemote(wg *sync.WaitGroup) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	buf := make([]byte, 65000)
	defer wg.Done()
	for {
		select {
		case <-p.Ctx.Done():
			return
		default:

			n, err := p.LocalConn.Read(buf)
			if err != nil {
				log.Println("ERRR READ: ", err)
				continue
			}

			// if _, found := common.GetPeer(p.Config.RemoteKey); !found {
			// 	log.Printf("Peer: %s not found in config\n", p.Config.RemoteKey)
			// 	p.Close()
			// 	return
			// }
			go func(n int, peerKey string) {
				metrics.MetricsMapLock.Lock()
				metric := metrics.MetricsMap[peerKey]
				metric.TrafficSent += float64(n) / (1 << 20)
				metrics.MetricsMap[peerKey] = metric
				metrics.MetricsMapLock.Unlock()
			}(n, p.Config.RemoteKey.String())

			//var srcPeerKeyHash, dstPeerKeyHash string
			if !p.Config.IsExtClient {
				buf, n, _, _ = packet.ProcessPacketBeforeSending(buf, n, p.Config.WgInterface.Device.PublicKey.String(), p.Config.RemoteKey.String())
				if err != nil {
					log.Println("failed to process pkt before sending: ", err)
				}
			}

			// log.Printf("PROXING TO REMOTE!!!---> %s >>>>> %s >>>>> %s [[ SrcPeerHash: %s, DstPeerHash: %s ]]\n",
			// 	p.LocalConn.LocalAddr(), server.NmProxyServer.Server.LocalAddr().String(), p.RemoteConn.String(), srcPeerKeyHash, dstPeerKeyHash)

			_, err = server.NmProxyServer.Server.WriteToUDP(buf[:n], p.RemoteConn)
			if err != nil {
				log.Println("Failed to send to remote: ", err)
			}

		}
	}

}

func (p *Proxy) Reset() {
	p.Close()
	if err := p.pullLatestConfig(); err != nil {
		log.Println("couldn't perform reset: ", err)
		return
	}
	p.Start()

}

func (p *Proxy) pullLatestConfig() error {
	peer, found := common.GetPeer(p.Config.RemoteKey)
	if found {
		p.Config.PeerEndpoint.Port = peer.Config.PeerEndpoint.Port
	} else {
		return errors.New("peer not found")
	}
	return nil

}

func (p *Proxy) startMetricsThread(wg *sync.WaitGroup, rTicker *time.Ticker) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	defer wg.Done()
	for {
		select {
		case <-p.Ctx.Done():
			return
		case <-ticker.C:
			metrics.MetricsMapLock.Lock()
			metric := metrics.MetricsMap[p.Config.RemoteKey.String()]
			if metric.ConnectionStatus {
				rTicker.Reset(*p.Config.PersistentKeepalive)
			}
			metric.ConnectionStatus = false
			metrics.MetricsMap[p.Config.RemoteKey.String()] = metric
			metrics.MetricsMapLock.Unlock()
			pkt, err := packet.CreateMetricPacket(uuid.New().ID(), p.Config.LocalKey, p.Config.RemoteKey)
			if err == nil {
				log.Printf("-----------> ##### $$$$$ SENDING METRIC PACKET TO: %s\n", p.RemoteConn.String())
				_, err = server.NmProxyServer.Server.WriteToUDP(pkt, p.RemoteConn)
				if err != nil {
					log.Println("Failed to send to metric pkt: ", err)
				}

			}
		}
	}
}

func (p *Proxy) peerUpdates(wg *sync.WaitGroup, ticker *time.Ticker) {
	defer wg.Done()
	for {
		select {
		case <-p.Ctx.Done():
			return
		case <-ticker.C:
			// send listen port packet
			m := &packet.ProxyUpdateMessage{
				Type:       packet.MessageProxyType,
				Action:     packet.UpdateListenPort,
				Sender:     p.Config.LocalKey,
				Reciever:   p.Config.RemoteKey,
				ListenPort: uint32(stun.Host.PrivPort),
			}
			pkt, err := packet.CreateProxyUpdatePacket(m)
			if err == nil {
				log.Printf("-----------> ##### $$$$$ SENDING Proxy Update PACKET TO: %s\n", p.RemoteConn.String())
				_, err = server.NmProxyServer.Server.WriteToUDP(pkt, p.RemoteConn)
				if err != nil {
					log.Println("Failed to send to metric pkt: ", err)
				}

			}
		}
	}
}

// ProxyPeer proxies everything from Wireguard to the RemoteKey peer and vice-versa
func (p *Proxy) ProxyPeer() {
	ticker := time.NewTicker(*p.Config.PersistentKeepalive)
	defer ticker.Stop()
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go p.proxyToRemote(wg)
	// if common.BehindNAT {
	wg.Add(1)
	go p.startMetricsThread(wg, ticker)
	wg.Add(1)
	go p.peerUpdates(wg, ticker)
	// }
	wg.Wait()

}

func (p *Proxy) updateEndpoint() error {
	udpAddr, err := net.ResolveUDPAddr("udp", p.LocalConn.LocalAddr().String())
	if err != nil {
		return err
	}
	// add local proxy connection as a Wireguard peer
	log.Printf("---> ####### Updating Peer:  %+v\n", p.Config.PeerConf)
	err = p.Config.WgInterface.UpdatePeer(p.Config.RemoteKey.String(), p.Config.PeerConf.AllowedIPs, wg.DefaultWgKeepAlive,
		udpAddr, p.Config.PeerConf.PresharedKey)
	if err != nil {
		return err
	}

	return nil
}

func GetFreeIp(cidrAddr string, dstPort int) (string, error) {
	//ensure AddressRange is valid
	if dstPort == 0 {
		return "", errors.New("dst port should be set")
	}
	if _, _, err := net.ParseCIDR(cidrAddr); err != nil {
		log.Println("UniqueAddress encountered  an error")
		return "", err
	}
	net4 := iplib.Net4FromStr(cidrAddr)
	newAddrs := net4.FirstAddress()
	for {
		if runtime.GOOS == "darwin" {
			_, err := common.RunCmd(fmt.Sprintf("ifconfig lo0 alias %s 255.255.255.255", newAddrs.String()), true)
			if err != nil {
				log.Println("Failed to add alias: ", err)
			}
		}

		conn, err := net.DialUDP("udp", &net.UDPAddr{
			IP:   net.ParseIP(newAddrs.String()),
			Port: models.NmProxyPort,
		}, &net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: dstPort,
		})
		if err != nil {
			log.Println("----> GetFreeIP ERR: ", err)
			if strings.Contains(err.Error(), "can't assign requested address") ||
				strings.Contains(err.Error(), "address already in use") || strings.Contains(err.Error(), "cannot assign requested address") {
				var nErr error
				newAddrs, nErr = net4.NextIP(newAddrs)
				if nErr != nil {
					return "", nErr
				}
			} else {
				return "", err
			}
		}
		if err == nil {
			conn.Close()
			return newAddrs.String(), nil
		}

	}
}
