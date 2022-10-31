package packet

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"log"
)

var udpHeaderLen = 8

func ProcessPacketBeforeSending(buf []byte, srckey string, n, dstPort int) ([]byte, int, error) {
	log.Println("@###### DST Port: ", dstPort)
	portbuf := new(bytes.Buffer)
	binary.Write(portbuf, binary.BigEndian, uint16(dstPort))
	hmd5 := md5.Sum([]byte(srckey))
	log.Printf("---> HASH: %x ", hmd5)
	if n > len(buf)-18 {
		buf = append(buf, portbuf.Bytes()[0])
		buf = append(buf, portbuf.Bytes()[1])
		buf = append(buf, hmd5[:]...)
	} else {
		buf[n] = portbuf.Bytes()[0]
		buf[n+1] = portbuf.Bytes()[1]
		copy(buf[n+2:n+2+len(hmd5)], hmd5[:])
	}

	n += 2
	n += len(hmd5)

	return buf, n, nil
}

func ExtractInfo(buffer []byte, n int) (int, int, string, error) {
	data := buffer[:n]
	var localWgPort uint16
	portBuf := data[n-18 : n-18+3]
	keyHash := data[n-16:]
	reader := bytes.NewReader(portBuf)
	err := binary.Read(reader, binary.BigEndian, &localWgPort)
	if err != nil {
		log.Println("Failed to read port buffer: ", err)
	}
	n -= 18
	return int(localWgPort), n, fmt.Sprintf("%x", keyHash), err
}
