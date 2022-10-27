package stun

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"gortc.io/stun"
)

type HostInfo struct {
	PublicIp net.IP
	PrivIp   net.IP
	PubPort  int
	PrivPort int
}

var Host HostInfo

func GetHostInfo() (info HostInfo) {

	s, err := net.ResolveUDPAddr("udp", "stun.nm.134.209.115.146.nip.io:3478")
	if err != nil {
		log.Fatal("Resolve: ", err)
	}
	l := &net.UDPAddr{
		IP:   net.ParseIP(""),
		Port: 51722,
	}
	conn, err := net.DialUDP("udp", l, s)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	fmt.Printf("%+v\n", conn.LocalAddr())
	c, err := stun.NewClient(conn)
	if err != nil {
		panic(err)
	}
	defer c.Close()
	re := strings.Split(conn.LocalAddr().String(), ":")
	info.PrivIp = net.ParseIP(re[0])
	info.PrivPort, _ = strconv.Atoi(re[1])
	// Building binding request with random transaction id.
	message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	//fmt.Printf("MESG: %+v\n", message)
	// Sending request to STUN server, waiting for response message.
	if err := c.Do(message, func(res stun.Event) {
		//fmt.Printf("RESP: %+v\n", res)
		if res.Error != nil {
			panic(res.Error)
		}
		// Decoding XOR-MAPPED-ADDRESS attribute from message.
		var xorAddr stun.XORMappedAddress
		if err := xorAddr.GetFrom(res.Message); err != nil {
			panic(err)
		}
		info.PublicIp = xorAddr.IP
		info.PubPort = xorAddr.Port
	}); err != nil {
		panic(err)
	}
	return
}
