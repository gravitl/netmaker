package packet

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"log"
	"net"
	"time"

	"github.com/gravitl/netmaker/nm-proxy/common"
	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/poly1305"
	"golang.zx2c4.com/wireguard/tai64n"
)

var (
	InitialChainKey [blake2s.Size]byte
	InitialHash     [blake2s.Size]byte
	ZeroNonce       [chacha20poly1305.NonceSize]byte
)

func init() {
	InitialChainKey = blake2s.Sum256([]byte(NoiseConstruction))
	mixHash(&InitialHash, &InitialChainKey, []byte(WGIdentifier))
}

type MessageInitiation struct {
	Type      uint32
	Sender    uint32
	Ephemeral NoisePublicKey
	Static    [NoisePublicKeySize + poly1305.TagSize]byte
	Timestamp [tai64n.TimestampSize + poly1305.TagSize]byte
	MAC1      [blake2s.Size128]byte
	MAC2      [blake2s.Size128]byte
}

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
