package packet

import (
	"crypto/md5"
	"fmt"
	"log"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

var udpHeaderLen = 8

func ProcessPacketBeforeSending(buf []byte, n int, srckey, dstKey string) ([]byte, int, string, string) {

	srcKeymd5 := md5.Sum([]byte(srckey))
	dstKeymd5 := md5.Sum([]byte(dstKey))
	if n > len(buf)-len(srcKeymd5)-len(dstKeymd5) {
		buf = append(buf, srcKeymd5[:]...)
		buf = append(buf, dstKeymd5[:]...)
	} else {
		copy(buf[n:n+len(srcKeymd5)], srcKeymd5[:])
		copy(buf[n+len(srcKeymd5):n+len(srcKeymd5)+len(dstKeymd5)], dstKeymd5[:])
	}
	n += len(srcKeymd5)
	n += len(dstKeymd5)

	return buf, n, fmt.Sprintf("%x", srcKeymd5), fmt.Sprintf("%x", dstKeymd5)
}

func ExtractInfo(buffer []byte, n int) (int, string, string) {
	data := buffer[:n]
	if len(data) < 32 {
		return 0, "", ""
	}
	srcKeyHash := data[n-32 : n-16]
	dstKeyHash := data[n-16:]
	n -= 32
	return n, fmt.Sprintf("%x", srcKeyHash), fmt.Sprintf("%x", dstKeyHash)
}

func StartSniffer(ifaceName string, extClient string) {
	var (
		snapshotLen int32 = 1024
		promiscuous bool  = false
		err         error
		timeout     time.Duration = 30 * time.Second
		handle      *pcap.Handle
	)
	// Open device
	handle, err = pcap.OpenLive(ifaceName, snapshotLen, promiscuous, timeout)
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()

	// var tcp layers.TCP
	// var icmp layers.ICMPv4
	// var udp layers.UDP
	// parser := gopacket.NewDecodingLayerParser(layers.LayerTypeIPv4, &udp, &tcp, &icmp)

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for {
		packet, err := packetSource.NextPacket()
		if err == nil {
			printPacketInfo(packet)
		}

	}
}

func printPacketInfo(packet gopacket.Packet) {
	// Let's see if the packet is an ethernet packet
	// ethernetLayer := packet.Layer(layers.LayerTypeEthernet)
	// if ethernetLayer != nil {
	// 	fmt.Println("Ethernet layer detected.")
	// 	ethernetPacket, _ := ethernetLayer.(*layers.Ethernet)
	// 	fmt.Println("Source MAC: ", ethernetPacket.SrcMAC)
	// 	fmt.Println("Destination MAC: ", ethernetPacket.DstMAC)
	// 	// Ethernet type is typically IPv4 but could be ARP or other
	// 	fmt.Println("Ethernet type: ", ethernetPacket.EthernetType)
	// 	fmt.Println()
	// }

	// Let's see if the packet is IP (even though the ether type told us)
	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	if ipLayer != nil {
		fmt.Println("IPv4 layer detected.")
		ip, _ := ipLayer.(*layers.IPv4)

		// IP layer variables:
		// Version (Either 4 or 6)
		// IHL (IP Header Length in 32-bit words)
		// TOS, Length, Id, Flags, FragOffset, TTL, Protocol (TCP?),
		// Checksum, SrcIP, DstIP
		fmt.Printf("From %s to %s\n", ip.SrcIP, ip.DstIP)
		fmt.Println("Protocol: ", ip.Protocol)
		fmt.Println()
	}

	// udpLayer := packet.Layer(layers.LayerTypeUDP)
	// if udpLayer != nil {
	// 	udp, _ := udpLayer.(*layers.UDP)
	// 	fmt.Printf("UDP: From port %d to %d\n", udp.SrcPort, udp.DstPort)
	// 	fmt.Println()
	// }

	// // Iterate over all layers, printing out each layer type
	// fmt.Println("All packet layers:")
	// for _, layer := range packet.Layers() {
	// 	fmt.Println("- ", layer.LayerType())
	// }

	// When iterating through packet.Layers() above,
	// if it lists Payload layer then that is the same as
	// this applicationLayer. applicationLayer contains the payload
	// applicationLayer := packet.ApplicationLayer()
	// if applicationLayer != nil {
	// 	fmt.Println("Application layer/Payload found.")
	// 	fmt.Printf("%s\n", applicationLayer.Payload())

	// 	// Search for a string inside the payload
	// 	if strings.Contains(string(applicationLayer.Payload()), "HTTP") {
	// 		fmt.Println("HTTP found!")
	// 	}
	// }

	// Check for errors
	if err := packet.ErrorLayer(); err != nil {
		fmt.Println("Error decoding some part of the packet:", err)
	}
}
