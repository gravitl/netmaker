package packet

import (
	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.zx2c4.com/wireguard/tai64n"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const poly1305TagSize = 16

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
	Type      MessageType
	Sender    uint32
	Ephemeral NoisePublicKey
	Static    [NoisePublicKeySize + poly1305TagSize]byte
	Timestamp [tai64n.TimestampSize + poly1305TagSize]byte
	MAC1      [blake2s.Size128]byte
	MAC2      [blake2s.Size128]byte
}

type MetricMessage struct {
	Type      MessageType
	ID        uint32
	Sender    wgtypes.Key
	Reciever  wgtypes.Key
	TimeStamp int64
}

type ProxyMessage struct {
	Type     MessageType
	Sender   [16]byte
	Reciever [16]byte
}

type ProxyUpdateMessage struct {
	Type       MessageType
	Action     ProxyActionType
	Sender     wgtypes.Key
	Reciever   wgtypes.Key
	ListenPort uint32
}
