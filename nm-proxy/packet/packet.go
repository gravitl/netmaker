package packet

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/gravitl/netmaker/nm-proxy/common"
	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func ConsumeHandshakeInitiationMsg(initiator bool, buf []byte, src *net.UDPAddr, devicePubKey NoisePublicKey, devicePrivKey NoisePrivateKey) error {

	var (
		hash     [blake2s.Size]byte
		chainKey [blake2s.Size]byte
	)
	var err error
	var msg MessageInitiation
	reader := bytes.NewReader(buf[:])
	err = binary.Read(reader, binary.LittleEndian, &msg)
	if err != nil {
		log.Println("Failed to decode initiation message")
		return err
	}

	if msg.Type != MessageInitiationType {
		return errors.New("not handshake initiation message")
	}
	log.Println("-----> ConsumeHandshakeInitiationMsg, Intitator:  ", initiator)
	mixHash(&hash, &InitialHash, devicePubKey[:])
	mixHash(&hash, &hash, msg.Ephemeral[:])
	mixKey(&chainKey, &InitialChainKey, msg.Ephemeral[:])

	// decrypt static key
	var peerPK NoisePublicKey
	var key [chacha20poly1305.KeySize]byte
	ss := sharedSecret(&devicePrivKey, msg.Ephemeral)
	if isZero(ss[:]) {
		return errors.New("no secret")
	}
	KDF2(&chainKey, &key, chainKey[:], ss[:])
	aead, _ := chacha20poly1305.New(key[:])
	_, err = aead.Open(peerPK[:0], ZeroNonce[:], msg.Static[:], hash[:])
	if err != nil {
		return err
	}
	log.Println("--------> Got HandShake from peer: ", base64.StdEncoding.EncodeToString(peerPK[:]), src)
	if val, ok := common.ExtClientsWaitTh[base64.StdEncoding.EncodeToString(peerPK[:])]; ok {
		val.CommChan <- src
		time.Sleep(time.Second * 3)
	}

	setZero(hash[:])
	setZero(chainKey[:])
	return nil
}

func CreateProxyUpdatePacket(msg *ProxyUpdateMessage) ([]byte, error) {
	var buff [MessageProxyUpdateSize]byte
	writer := bytes.NewBuffer(buff[:0])
	err := binary.Write(writer, binary.LittleEndian, msg)
	if err != nil {
		return nil, err
	}
	packet := writer.Bytes()
	return packet, nil

}

func ConsumeProxyUpdateMsg(buf []byte) (*ProxyUpdateMessage, error) {
	var msg ProxyUpdateMessage
	reader := bytes.NewReader(buf[:])
	err := binary.Read(reader, binary.LittleEndian, &msg)
	if err != nil {
		log.Println("Failed to decode proxy update message")
		return nil, err
	}

	if msg.Type != MessageProxyUpdateType {
		return nil, errors.New("not proxy update message")
	}
	return &msg, nil
}

func CreateMetricPacket(id uint32, sender, reciever wgtypes.Key) ([]byte, error) {
	msg := MetricMessage{
		Type:      MessageMetricsType,
		ID:        id,
		Sender:    sender,
		Reciever:  reciever,
		TimeStamp: time.Now().UnixMilli(),
	}
	log.Printf("----------> $$$$$$ CREATED PACKET: %+v\n", msg)
	var buff [MessageMetricSize]byte
	writer := bytes.NewBuffer(buff[:0])
	err := binary.Write(writer, binary.LittleEndian, msg)
	if err != nil {
		return nil, err
	}
	packet := writer.Bytes()
	return packet, nil
}

func ConsumeMetricPacket(buf []byte) (*MetricMessage, error) {
	var msg MetricMessage
	var err error
	reader := bytes.NewReader(buf[:])
	err = binary.Read(reader, binary.LittleEndian, &msg)
	if err != nil {
		log.Println("Failed to decode metric message")
		return nil, err
	}

	if msg.Type != MessageMetricsType {
		return nil, errors.New("not  metric message")
	}
	return &msg, nil
}

func ProcessPacketBeforeSending(buf []byte, n int, srckey, dstKey string) ([]byte, int, string, string) {

	srcKeymd5 := md5.Sum([]byte(srckey))
	dstKeymd5 := md5.Sum([]byte(dstKey))
	m := ProxyMessage{
		Type:     MessageProxyType,
		Sender:   srcKeymd5,
		Reciever: dstKeymd5,
	}
	var msgBuffer [MessageProxySize]byte
	writer := bytes.NewBuffer(msgBuffer[:0])
	err := binary.Write(writer, binary.LittleEndian, m)
	if err != nil {
		log.Println(err)
	}
	if n > len(buf)-MessageProxySize {
		buf = append(buf, msgBuffer[:]...)

	} else {
		copy(buf[n:n+MessageProxySize], msgBuffer[:])
	}
	n += MessageProxySize

	return buf, n, fmt.Sprintf("%x", srcKeymd5), fmt.Sprintf("%x", dstKeymd5)
}

func ExtractInfo(buffer []byte, n int) (int, string, string, error) {
	data := buffer[:n]
	if len(data) < MessageProxySize {
		return n, "", "", errors.New("proxy message not found")
	}
	var msg ProxyMessage
	var err error
	reader := bytes.NewReader(buffer[n-MessageProxySize:])
	err = binary.Read(reader, binary.LittleEndian, &msg)
	if err != nil {
		log.Println("Failed to decode proxy message")
		return n, "", "", err
	}

	if msg.Type != MessageProxyType {
		return n, "", "", errors.New("not a proxy message")
	}
	n -= MessageProxySize
	return n, fmt.Sprintf("%x", msg.Sender), fmt.Sprintf("%x", msg.Reciever), nil
}
