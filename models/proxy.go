package models

import (
	"net"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// ProxyAction - type for proxy action
type ProxyAction string

const (
	// default proxy port
	NmProxyPort = 51722
	// PersistentKeepaliveInterval - default keepalive for wg peer
	DefaultPersistentKeepaliveInterval = time.Duration(time.Second * 20)

	// ProxyUpdate - constant for proxy update action
	ProxyUpdate ProxyAction = "PROXY_UPDATE"
	// ProxyDeletePeers - constant for proxy delete peers action
	ProxyDeletePeers ProxyAction = "PROXY_DELETE"
	// ProxyDeleteAllPeers - constant for proxy delete all peers action
	ProxyDeleteAllPeers ProxyAction = "PROXY_DELETE_ALL"
	// NoProxy - constant for no ProxyAction
	NoProxy ProxyAction = "NO_PROXY"
)

// RelayedConf - struct relayed peers config
type RelayedConf struct {
	RelayedPeerEndpoint *net.UDPAddr         `json:"relayed_peer_endpoint"`
	RelayedPeerPubKey   string               `json:"relayed_peer_pub_key"`
	Peers               []wgtypes.PeerConfig `json:"relayed_peers"`
}

// PeerConf - struct for peer config in the network
type PeerConf struct {
	Proxy            bool         `json:"proxy"`
	PublicListenPort int32        `json:"public_listen_port"`
	ProxyListenPort  int          `json:"proxy_listen_port"`
	IsExtClient      bool         `json:"is_ext_client"`
	Address          net.IP       `json:"address"`
	ExtInternalIp    net.IP       `json:"ext_internal_ip"`
	IsRelayed        bool         `json:"is_relayed"`
	RelayedTo        *net.UDPAddr `json:"relayed_to"`
	NatType          string       `json:"nat_type"`
}

// ProxyManagerPayload - struct for proxy manager payload
type ProxyManagerPayload struct {
	Action        ProxyAction `json:"action"`
	InterfaceName string      `json:"interface_name"`
	Server        string      `json:"server"`
	//WgAddr          string                 `json:"wg_addr"`
	Peers           []wgtypes.PeerConfig   `json:"peers"`
	PeerMap         map[string]PeerConf    `json:"peer_map"`
	IsIngress       bool                   `json:"is_ingress"`
	IsRelayed       bool                   `json:"is_relayed"`
	RelayedTo       *net.UDPAddr           `json:"relayed_to"`
	IsRelay         bool                   `json:"is_relay"`
	RelayedPeerConf map[string]RelayedConf `json:"relayed_conf"`
}

// Metric - struct for metric data
type ProxyMetric struct {
	NodeConnectionStatus map[string]bool `json:"node_connection_status"`
	LastRecordedLatency  uint64          `json:"last_recorded_latency"`
	TrafficSent          int64           `json:"traffic_sent"`     // stored in MB
	TrafficRecieved      int64           `json:"traffic_recieved"` // stored in MB
}
