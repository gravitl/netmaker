package packet

import (
	"bytes"
	"encoding/binary"
	"log"
)

var udpHeaderLen = 8

func ProcessPacketBeforeSending(buf []byte, n, dstPort int) ([]byte, int, error) {
	log.Println("@###### DST Port: ", dstPort)
	portbuf := new(bytes.Buffer)
	binary.Write(portbuf, binary.BigEndian, uint16(dstPort))
	if n > len(buf)-2 {
		buf = append(buf, portbuf.Bytes()[0])
		buf = append(buf, portbuf.Bytes()[1])
	} else {
		buf[n] = portbuf.Bytes()[0]
		buf[n+1] = portbuf.Bytes()[1]
	}

	n += 2

	return buf, n, nil
}

func ExtractInfo(buffer []byte, n int) (int, int, error) {
	data := buffer[:n]
	var localWgPort uint16
	portBuf := data[n-2 : n+1]
	reader := bytes.NewReader(portBuf)
	err := binary.Read(reader, binary.BigEndian, &localWgPort)
	if err != nil {
		log.Println("Failed to read port buffer: ", err)
	}
	n -= 2
	return int(localWgPort), n, err
}
