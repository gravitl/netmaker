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
	"golang.zx2c4.com/wireguard/tai64n"
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

func CreateMetricPacket(id uint64, sender, reciever NoisePublicKey) ([]byte, error) {
	msg := MetricMessage{
		ID:        id,
		Sender:    sender,
		Reciever:  reciever,
		TimeStamp: tai64n.Now(),
	}
	var buff [MessageMetricSize]byte
	writer := bytes.NewBuffer(buff[:0])
	err := binary.Write(writer, binary.LittleEndian, msg)
	if err != nil {
		return nil, err
	}
	packet := writer.Bytes()
	return packet, nil
}

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
