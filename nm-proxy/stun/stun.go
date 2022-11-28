package stun

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/gravitl/netmaker/nm-proxy/models"
	"gortc.io/stun"
)

type HostInfo struct {
	PublicIp net.IP
	PrivIp   net.IP
	PubPort  int
	PrivPort int
}

var Host HostInfo

func GetHostInfo(stunHostAddr string) (info HostInfo) {

	s, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:3478", stunHostAddr))
	if err != nil {
		log.Println("Resolve: ", err)
		return
	}
	l := &net.UDPAddr{
		IP:   net.ParseIP(""),
		Port: models.NmProxyPort,
	}
	conn, err := net.DialUDP("udp", l, s)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()
	c, err := stun.NewClient(conn)
	if err != nil {
		log.Println(err)
		return
	}
	defer c.Close()
	re := strings.Split(conn.LocalAddr().String(), ":")
	info.PrivIp = net.ParseIP(re[0])
	info.PrivPort, _ = strconv.Atoi(re[1])
	// Building binding request with random transaction id.
	message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	// Sending request to STUN server, waiting for response message.
	if err := c.Do(message, func(res stun.Event) {
		if res.Error != nil {
			log.Println("stun error: ", res.Error)
			return
		}
		// Decoding XOR-MAPPED-ADDRESS attribute from message.
		var xorAddr stun.XORMappedAddress
		if err := xorAddr.GetFrom(res.Message); err != nil {
			log.Println("stun error: ", res.Error)
			return
		}
		info.PublicIp = xorAddr.IP
		info.PubPort = xorAddr.Port
	}); err != nil {
		log.Println("stun error: ", err)
	}
	return
}
