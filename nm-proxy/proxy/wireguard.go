package proxy

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"runtime"
	"strings"

	"github.com/c-robinson/iplib"
	"github.com/gravitl/netmaker/nm-proxy/common"
	"github.com/gravitl/netmaker/nm-proxy/packet"
	"github.com/gravitl/netmaker/nm-proxy/server"
	"github.com/gravitl/netmaker/nm-proxy/wg"
)

func NewProxy(config Config) *Proxy {
	p := &Proxy{Config: config}
	p.Ctx, p.Cancel = context.WithCancel(context.Background())
	return p
}

// proxyToRemote proxies everything from Wireguard to the RemoteKey peer
func (p *Proxy) ProxyToRemote() {

	go func() {
		<-p.Ctx.Done()
		log.Println("Closing connection for: ", p.LocalConn.LocalAddr().String())
		p.LocalConn.Close()
	}()
	buf := make([]byte, 65000)
	for {
		select {
		case <-p.Ctx.Done():
			log.Printf("----------> stopped proxying to remote peer %s due to closed connection\n", p.Config.RemoteKey)
			if runtime.GOOS == "darwin" {
				host, _, err := net.SplitHostPort(p.LocalConn.LocalAddr().String())
				if err != nil {
					log.Println("Failed to split host: ", p.LocalConn.LocalAddr().String(), err)
					return
				}

				if host != "127.0.0.1" {
					_, err = common.RunCmd(fmt.Sprintf("ifconfig lo0 -alias %s 255.255.255.255", host), true)
					if err != nil {
						log.Println("Failed to add alias: ", err)
					}
				}

			}

			return
		default:

			n, err := p.LocalConn.Read(buf)
			if err != nil {
				log.Println("ERRR READ: ", err)
				continue
			}
			//go func(buf []byte, n int) {
			ifaceConf := common.WgIFaceMap[p.Config.WgInterface.Name]
			if peerI, ok := ifaceConf.PeerMap[p.Config.RemoteKey]; ok {
				var srcPeerKeyHash, dstPeerKeyHash string
				buf, n, srcPeerKeyHash, dstPeerKeyHash = packet.ProcessPacketBeforeSending(buf, n, peerI.Config.LocalKey, peerI.Config.Key)
				if err != nil {
					log.Println("failed to process pkt before sending: ", err)
				}
				log.Printf("PROXING TO REMOTE!!!---> %s >>>>> %s >>>>> %s [[ SrcPeerHash: %s, DstPeerHash: %s ]]\n",
					p.LocalConn.LocalAddr(), server.NmProxyServer.Server.LocalAddr().String(), p.RemoteConn.String(), srcPeerKeyHash, dstPeerKeyHash)
			} else {
				log.Printf("Peer: %s not found in config\n", p.Config.RemoteKey)
				p.Cancel()
				return
			}
			//test(n, buf)

			_, err = server.NmProxyServer.Server.WriteToUDP(buf[:n], p.RemoteConn)
			if err != nil {
				log.Println("Failed to send to remote: ", err)
			}
			//}(buf, n)

		}
	}
}
func test(n int, buffer []byte) {
	data := buffer[:n]
	srcKeyHash := data[n-32 : n-16]
	dstKeyHash := data[n-16:]
	log.Printf("--------> TEST PACKET [ SRCKEYHASH: %x ], [ DSTKEYHASH: %x ] \n", srcKeyHash, dstKeyHash)
}

func (p *Proxy) updateEndpoint() error {
	udpAddr, err := net.ResolveUDPAddr("udp", p.LocalConn.LocalAddr().String())
	if err != nil {
		return err
	}
	// add local proxy connection as a Wireguard peer
	log.Printf("---> ####### Updating Peer:  %+v\n", p.Config.PeerConf)
	err = p.Config.WgInterface.UpdatePeer(p.Config.RemoteKey, p.Config.PeerConf.AllowedIPs, wg.DefaultWgKeepAlive,
		udpAddr, p.Config.PeerConf.PresharedKey)
	if err != nil {
		return err
	}

	return nil
}

func (p *Proxy) Start(remoteConn *net.UDPAddr) error {
	p.RemoteConn = remoteConn

	var err error

	//log.Printf("----> WGIFACE: %+v\n", p.Config.WgInterface)
	addr, err := GetFreeIp(common.DefaultCIDR, p.Config.WgInterface.Port)
	if err != nil {
		log.Println("Failed to get freeIp: ", err)
		return err
	}
	wgListenAddr, err := GetInterfaceListenAddr(p.Config.WgInterface.Port)
	if err != nil {
		log.Println("failed to get wg listen addr: ", err)
		return err
	}
	if runtime.GOOS == "darwin" {
		wgListenAddr.IP = net.ParseIP(addr)
	}
	//log.Println("--------->#### Wg Listen Addr: ", wgListenAddr.String())
	p.LocalConn, err = net.DialUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP(addr),
		Port: common.NmProxyPort,
	}, wgListenAddr)
	if err != nil {
		log.Printf("failed dialing to local Wireguard port,Err: %v\n", err)
		return err
	}

	log.Printf("Dialing to local Wireguard port %s --> %s\n", p.LocalConn.LocalAddr().String(), p.LocalConn.RemoteAddr().String())
	err = p.updateEndpoint()
	if err != nil {
		log.Printf("error while updating Wireguard peer endpoint [%s] %v\n", p.Config.RemoteKey, err)
		return err
	}

	go p.ProxyToRemote()

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
	log.Println("COUNT: ", net4.Count())
	for {
		if runtime.GOOS == "darwin" {
			_, err := common.RunCmd(fmt.Sprintf("ifconfig lo0 alias %s 255.255.255.255", newAddrs.String()), true)
			if err != nil {
				log.Println("Failed to add alias: ", err)
			}
		}

		conn, err := net.DialUDP("udp", &net.UDPAddr{
			IP:   net.ParseIP(newAddrs.String()),
			Port: common.NmProxyPort,
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
