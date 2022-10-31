package proxy

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"runtime"
	"strconv"
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
	buf := make([]byte, 1500)

	go func() {
		<-p.Ctx.Done()
		defer p.LocalConn.Close()
		defer p.RemoteConn.Close()
	}()
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
				if host == "127.0.0.1" {
					return
				}
				_, err = common.RunCmd(fmt.Sprintf("ifconfig lo0 -alias %s 255.255.255.255", host), true)
				if err != nil {
					log.Println("Failed to add alias: ", err)
				}
			}
			return
		default:

			n, err := p.LocalConn.Read(buf)
			if err != nil {
				log.Println("ERRR READ: ", err)
				continue
			}
			peers := common.WgIFaceMap[p.Config.WgInterface.Name]
			if peerI, ok := peers[p.Config.RemoteKey]; ok {
				log.Println("PROCESSING PKT BEFORE SENDING")

				buf, n, err = packet.ProcessPacketBeforeSending(buf, peerI.Config.LocalKey, n, peerI.Config.RemoteWgPort)
				if err != nil {
					log.Println("failed to process pkt before sending: ", err)
				}
			} else {
				log.Printf("Peer: %s not found in config\n", p.Config.RemoteKey)
			}
			// test(n, buf)
			log.Printf("PROXING TO REMOTE!!!---> %s >>>>> %s\n", server.NmProxyServer.Server.LocalAddr().String(), p.RemoteConn.RemoteAddr().String())
			host, port, _ := net.SplitHostPort(p.RemoteConn.RemoteAddr().String())
			portInt, _ := strconv.Atoi(port)
			_, err = server.NmProxyServer.Server.WriteToUDP(buf[:n], &net.UDPAddr{
				IP:   net.ParseIP(host),
				Port: portInt,
			})
			if err != nil {
				log.Println("Failed to send to remote: ", err)
			}
		}
	}
}

func (p *Proxy) updateEndpoint() error {
	udpAddr, err := net.ResolveUDPAddr("udp", p.LocalConn.LocalAddr().String())
	if err != nil {
		return err
	}
	log.Println("--------> UDPADDR:  ", udpAddr)
	// add local proxy connection as a Wireguard peer
	err = p.Config.WgInterface.UpdatePeer(p.Config.RemoteKey, p.Config.AllowedIps, wg.DefaultWgKeepAlive,
		udpAddr, p.Config.PreSharedKey)
	if err != nil {
		return err
	}

	return nil
}

func (p *Proxy) Start(remoteConn net.Conn) error {
	p.RemoteConn = remoteConn

	var err error
	// err = p.Config.WgInterface.GetWgIface(p.Config.WgInterface.Name)
	// if err != nil {
	// 	log.Println("Failed to get iface: ", p.Config.WgInterface.Name, err)
	// 	return err
	// }
	// wgAddr, err := GetInterfaceIpv4Addr(p.Config.WgInterface.Name)
	// if err != nil {
	// 	log.Println("failed to get interface addr: ", err)
	// 	return err
	// }
	log.Printf("----> WGIFACE: %+v\n", p.Config.WgInterface)
	addr, err := GetFreeIp("127.0.0.1/8", p.Config.WgInterface.Port)
	if err != nil {
		log.Println("Failed to get freeIp: ", err)
		return err
	}
	wgAddr := "127.0.0.1"
	if runtime.GOOS == "darwin" {
		wgAddr = addr
	}

	p.LocalConn, err = net.DialUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP(addr),
		Port: common.NmProxyPort,
	}, &net.UDPAddr{
		IP:   net.ParseIP(wgAddr),
		Port: p.Config.WgInterface.Port,
	})
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
			if strings.Contains(err.Error(), "can't assign requested address") || strings.Contains(err.Error(), "address already in use") {
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
