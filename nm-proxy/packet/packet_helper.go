package packet

import (
	"bytes"
	"crypto/hmac"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"hash"
	"log"

	"github.com/gravitl/netmaker/nm-proxy/wg"
	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/poly1305"
	"golang.zx2c4.com/wireguard/tai64n"
)

func init() {
	InitialChainKey = blake2s.Sum256([]byte(NoiseConstruction))
	mixHash(&InitialHash, &InitialChainKey, []byte(WGIdentifier))
}
func mixKey(dst, c *[blake2s.Size]byte, data []byte) {
	KDF1(dst, c[:], data)
}

func mixHash(dst, h *[blake2s.Size]byte, data []byte) {
	hash, _ := blake2s.New256(nil)
	hash.Write(h[:])
	hash.Write(data)
	hash.Sum(dst[:0])
	hash.Reset()
}
func HMAC1(sum *[blake2s.Size]byte, key, in0 []byte) {
	mac := hmac.New(func() hash.Hash {
		h, _ := blake2s.New256(nil)
		return h
	}, key)
	mac.Write(in0)
	mac.Sum(sum[:0])
}

func HMAC2(sum *[blake2s.Size]byte, key, in0, in1 []byte) {
	mac := hmac.New(func() hash.Hash {
		h, _ := blake2s.New256(nil)
		return h
	}, key)
	mac.Write(in0)
	mac.Write(in1)
	mac.Sum(sum[:0])
}

func KDF1(t0 *[blake2s.Size]byte, key, input []byte) {
	HMAC1(t0, key, input)
	HMAC1(t0, t0[:], []byte{0x1})
}

const (
	NoiseConstruction = "Noise_IKpsk2_25519_ChaChaPoly_BLAKE2s"
	WGIdentifier      = "WireGuard v1 zx2c4 Jason@zx2c4.com"
	WGLabelMAC1       = "mac1----"
	WGLabelCookie     = "cookie--"
)

const (
	MessageInitiationType  = 1
	MessageResponseType    = 2
	MessageCookieReplyType = 3
	MessageTransportType   = 4
)
const (
	NoisePublicKeySize    = 32
	NoisePrivateKeySize   = 32
	NoisePresharedKeySize = 32
)

type (
	NoisePublicKey    [NoisePublicKeySize]byte
	NoisePrivateKey   [NoisePrivateKeySize]byte
	NoisePresharedKey [NoisePresharedKeySize]byte
	NoiseNonce        uint64 // padded to 12-bytes
)

type MessageInitiation struct {
	Type      uint32
	Sender    uint32
	Ephemeral NoisePublicKey
	Static    [NoisePublicKeySize + poly1305.TagSize]byte
	Timestamp [tai64n.TimestampSize + poly1305.TagSize]byte
	MAC1      [blake2s.Size128]byte
	MAC2      [blake2s.Size128]byte
}

var (
	InitialChainKey [blake2s.Size]byte
	InitialHash     [blake2s.Size]byte
	ZeroNonce       [chacha20poly1305.NonceSize]byte
)

func KDF2(t0, t1 *[blake2s.Size]byte, key, input []byte) {
	var prk [blake2s.Size]byte
	HMAC1(&prk, key, input)
	HMAC1(t0, prk[:], []byte{0x1})
	HMAC2(t1, prk[:], t0[:], []byte{0x2})
	setZero(prk[:])
}
func setZero(arr []byte) {
	for i := range arr {
		arr[i] = 0
	}
}
func isZero(val []byte) bool {
	acc := 1
	for _, b := range val {
		acc &= subtle.ConstantTimeByteEq(b, 0)
	}
	return acc == 1
}
func uapiCfg(cfg ...string) string {
	if len(cfg)%2 != 0 {
		panic("odd number of args to uapiReader")
	}
	buf := new(bytes.Buffer)
	for i, s := range cfg {
		buf.WriteString(s)
		sep := byte('\n')
		if i%2 == 0 {
			sep = '='
		}
		buf.WriteByte(sep)
	}
	return buf.String()
}
func sharedSecret(sk *NoisePrivateKey, pk NoisePublicKey) (ss [NoisePublicKeySize]byte) {
	apk := (*[NoisePublicKeySize]byte)(&pk)
	ask := (*[NoisePrivateKeySize]byte)(sk)
	curve25519.ScalarMult(&ss, ask, apk)
	return ss
}

func ConsumeHandshakeMsg(buf []byte, devicePubKey NoisePublicKey, devicePrivKey NoisePrivateKey) (string, error) {

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
		return "", err
	}

	if msg.Type != MessageInitiationType {
		return "", errors.New("not handshake initiate message")
	}
	log.Println("-----> HandSHAKE MESSAGE")
	mixHash(&hash, &InitialHash, devicePubKey[:])
	mixHash(&hash, &hash, msg.Ephemeral[:])
	mixKey(&chainKey, &InitialChainKey, msg.Ephemeral[:])

	// decrypt static key
	var peerPK NoisePublicKey
	var key [chacha20poly1305.KeySize]byte
	ss := sharedSecret(&devicePrivKey, msg.Ephemeral)
	if isZero(ss[:]) {
		return "", errors.New("no secret")
	}
	KDF2(&chainKey, &key, chainKey[:], ss[:])
	aead, _ := chacha20poly1305.New(key[:])
	_, err = aead.Open(peerPK[:0], ZeroNonce[:], msg.Static[:], hash[:])
	if err != nil {
		return "", err
	}
	setZero(hash[:])
	setZero(chainKey[:])
	return base64.StdEncoding.EncodeToString(peerPK[:]), nil
}

func loadExactHex(dst []byte, src string) error {
	slice, err := hex.DecodeString(src)
	if err != nil {
		return err
	}
	if len(slice) != len(dst) {
		return errors.New("hex string does not fit the slice")
	}
	copy(dst, slice)
	return nil
}

func (key NoisePrivateKey) IsZero() bool {
	var zero NoisePrivateKey
	return key.Equals(zero)
}

func (key NoisePrivateKey) Equals(tar NoisePrivateKey) bool {
	return subtle.ConstantTimeCompare(key[:], tar[:]) == 1
}

func (key *NoisePrivateKey) FromHex(src string) (err error) {
	err = loadExactHex(key[:], src)
	key.clamp()
	return
}

func (key *NoisePrivateKey) FromMaybeZeroHex(src string) (err error) {
	err = loadExactHex(key[:], src)
	if key.IsZero() {
		return
	}
	key.clamp()
	return
}
func (sk *NoisePrivateKey) clamp() {
	sk[0] &= 248
	sk[31] = (sk[31] & 127) | 64
}

func (key *NoisePublicKey) FromHex(src string) error {
	return loadExactHex(key[:], src)
}

func (key NoisePublicKey) IsZero() bool {
	var zero NoisePublicKey
	return key.Equals(zero)
}

func (key NoisePublicKey) Equals(tar NoisePublicKey) bool {
	return subtle.ConstantTimeCompare(key[:], tar[:]) == 1
}

func (key *NoisePresharedKey) FromHex(src string) error {
	return loadExactHex(key[:], src)
}

func GetDeviceKeys(ifaceName string) (NoisePrivateKey, NoisePublicKey, error) {
	wgPrivKey := wg.GetWgIfacePrivKey(ifaceName)
	wgPubKey := wg.GetWgIfacePubKey(ifaceName)

	return wgPrivKey, wgPubKey, nil
}
