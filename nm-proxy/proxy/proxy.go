package proxy

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"runtime"

	"github.com/gravitl/netmaker/nm-proxy/common"
	"github.com/gravitl/netmaker/nm-proxy/models"
)

// Proxy -  WireguardProxy proxies
type Proxy struct {
	Ctx        context.Context
	Cancel     context.CancelFunc
	Config     models.ProxyConfig
	RemoteConn *net.UDPAddr
	LocalConn  net.Conn
}

func (p *Proxy) Start() error {

	var err error
	p.RemoteConn = p.Config.PeerEndpoint
	log.Printf("----> Established Remote Conn with RPeer: %s, ----> RAddr: %s", p.Config.RemoteKey.String(), p.RemoteConn.String())
	addr, err := GetFreeIp(models.DefaultCIDR, p.Config.WgInterface.Port)
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
	p.LocalConn, err = net.DialUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP(addr),
		Port: models.NmProxyPort,
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
	localAddr, err := net.ResolveUDPAddr("udp", p.LocalConn.LocalAddr().String())
	if err != nil {
		log.Println("failed to resolve local addr: ", err)
		return err
	}
	p.Config.LocalConnAddr = localAddr
	p.Config.RemoteConnAddr = p.RemoteConn
	go p.ProxyPeer()

	return nil
}

func (p *Proxy) Close() {
	log.Println("------> Closing Proxy for ", p.Config.RemoteKey.String())
	p.Cancel()
	p.LocalConn.Close()
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
	close(p.Config.RecieverChan)
}

func GetInterfaceListenAddr(port int) (*net.UDPAddr, error) {
	locallistenAddr := "127.0.0.1"
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", locallistenAddr, port))
	if err != nil {
		return udpAddr, err
	}
	return udpAddr, nil
}

func getBoardCastAddress() ([]net.Addr, error) {
	localnets, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var (
		ief   net.Interface
		addrs []net.Addr
	)
	for _, ief = range localnets {
		if ief.Flags&net.FlagBroadcast != 0 && ief.Flags&net.FlagUp != 0 {
			addrs, err = ief.Addrs()
			if err == nil {
				return addrs, nil
			}

		}
	}
	return nil, errors.New("couldn't obtain the broadcast addr")
}

// func StartSniffer(ctx context.Context, ifaceName, ingGwAddr, extClientAddr string, port int) {
// 	log.Println("Starting Packet Sniffer for iface: ", ifaceName)
// 	var (
// 		snapshotLen int32 = 1024
// 		promiscuous bool  = false
// 		err         error
// 		timeout     time.Duration = 1 * time.Microsecond
// 		handle      *pcap.Handle
// 	)
// 	// Open device
// 	handle, err = pcap.OpenLive(ifaceName, snapshotLen, promiscuous, timeout)
// 	if err != nil {
// 		log.Println("failed to start sniffer for iface: ", ifaceName, err)
// 		return
// 	}
// 	// if err := handle.SetBPFFilter(fmt.Sprintf("src %s and port %d", extClientAddr, port)); err != nil {
// 	// 	log.Println("failed to set bpf filter: ", err)
// 	// 	return
// 	// }
// 	defer handle.Close()

// 	// var tcp layers.TCP
// 	// var icmp layers.ICMPv4
// 	// var udp layers.UDP
// 	// parser := gopacket.NewDecodingLayerParser(layers.LayerTypeIPv4, &udp, &tcp, &icmp)

// 	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
// 	for {
// 		select {
// 		case <-ctx.Done():
// 			log.Println("Stopping packet sniffer for iface: ", ifaceName, " port: ", port)
// 			return
// 		default:
// 			packet, err := packetSource.NextPacket()
// 			if err == nil {
// 				//processPkt(ifaceName, packet)
// 				ipLayer := packet.Layer(layers.LayerTypeIPv4)
// 				if ipLayer != nil {
// 					fmt.Println("IPv4 layer detected.")
// 					ip, _ := ipLayer.(*layers.IPv4)

// 					// IP layer variables:
// 					// Version (Either 4 or 6)
// 					// IHL (IP Header Length in 32-bit words)
// 					// TOS, Length, Id, Flags, FragOffset, TTL, Protocol (TCP?),
// 					// Checksum, SrcIP, DstIP
// 					fmt.Println("#########################")
// 					fmt.Printf("From %s to %s\n", ip.SrcIP, ip.DstIP)
// 					fmt.Println("Protocol: ", ip.Protocol.String())
// 					if (ip.SrcIP.String() == extClientAddr && ip.DstIP.String() != ingGwAddr) ||
// 						(ip.DstIP.String() == extClientAddr && ip.SrcIP.String() != ingGwAddr) {

// 						log.Println("-----> Fowarding PKT From: ", ip.SrcIP, " to: ", ip.DstIP)
// 						c, err := net.Dial("ip", ip.DstIP.String())
// 						if err == nil {
// 							c.Write(ip.Payload)
// 							c.Close()
// 						} else {
// 							log.Println("------> Failed to forward packet from sniffer: ", err)

// 						}
// 					}

// 					fmt.Println("#########################")
// 				}
// 			}
// 		}

// 	}
// }

// func processPkt(iface string, packet gopacket.Packet) {
// 	// Let's see if the packet is an ethernet packet
// 	// ethernetLayer := packet.Layer(layers.LayerTypeEthernet)
// 	// if ethernetLayer != nil {
// 	// 	fmt.Println("Ethernet layer detected.")
// 	// 	ethernetPacket, _ := ethernetLayer.(*layers.Ethernet)
// 	// 	fmt.Println("Source MAC: ", ethernetPacket.SrcMAC)
// 	// 	fmt.Println("Destination MAC: ", ethernetPacket.DstMAC)
// 	// 	// Ethernet type is typically IPv4 but could be ARP or other
// 	// 	fmt.Println("Ethernet type: ", ethernetPacket.EthernetType)
// 	// 	fmt.Println()
// 	// }

// 	// Let's see if the packet is IP (even though the ether type told us)
// 	ipLayer := packet.Layer(layers.LayerTypeIPv4)
// 	if ipLayer != nil {
// 		fmt.Println("IPv4 layer detected.")
// 		ip, _ := ipLayer.(*layers.IPv4)

// 		// IP layer variables:
// 		// Version (Either 4 or 6)
// 		// IHL (IP Header Length in 32-bit words)
// 		// TOS, Length, Id, Flags, FragOffset, TTL, Protocol (TCP?),
// 		// Checksum, SrcIP, DstIP
// 		fmt.Printf("From %s to %s\n", ip.SrcIP, ip.DstIP)
// 		fmt.Println("Protocol: ", ip.Protocol)
// 		fmt.Println()

// 	}

// 	// udpLayer := packet.Layer(layers.LayerTypeUDP)
// 	// if udpLayer != nil {
// 	// 	udp, _ := udpLayer.(*layers.UDP)
// 	// 	fmt.Printf("UDP: From port %d to %d\n", udp.SrcPort, udp.DstPort)
// 	// 	fmt.Println()
// 	// }

// 	// // Iterate over all layers, printing out each layer type
// 	// fmt.Println("All packet layers:")
// 	// for _, layer := range packet.Layers() {
// 	// 	fmt.Println("- ", layer.LayerType())
// 	// }

// 	// When iterating through packet.Layers() above,
// 	// if it lists Payload layer then that is the same as
// 	// this applicationLayer. applicationLayer contains the payload
// 	// applicationLayer := packet.ApplicationLayer()
// 	// if applicationLayer != nil {
// 	// 	fmt.Println("Application layer/Payload found.")
// 	// 	fmt.Printf("%s\n", applicationLayer.Payload())

// 	// 	// Search for a string inside the payload
// 	// 	if strings.Contains(string(applicationLayer.Payload()), "HTTP") {
// 	// 		fmt.Println("HTTP found!")
// 	// 	}
// 	// }

// 	// Check for errors
// 	if err := packet.ErrorLayer(); err != nil {
// 		fmt.Println("Error decoding some part of the packet:", err)
// 	}
// }
