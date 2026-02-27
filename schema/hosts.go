package schema

import (
	"net"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/datatypes"
)

// Iface struct for local interfaces of a node
type Iface struct {
	Name          string    `json:"name"`
	Address       net.IPNet `json:"address"`
	AddressString string    `json:"addressString"`
}

type Host struct {
	ID                  uuid.UUID                   `gorm:"primaryKey" json:"id" yaml:"id"`
	Verbosity           int                         `json:"verbosity" yaml:"verbosity"`
	FirewallInUse       string                      `json:"firewallinuse" yaml:"firewallinuse"`
	Version             string                      `json:"version" yaml:"version"`
	IPForwarding        bool                        `json:"ipforwarding" yaml:"ipforwarding"`
	DaemonInstalled     bool                        `json:"daemoninstalled" yaml:"daemoninstalled"`
	AutoUpdate          bool                        `json:"autoupdate" yaml:"autoupdate"`
	HostPass            string                      `json:"hostpass" yaml:"hostpass"`
	Name                string                      `json:"name" yaml:"name"`
	OS                  string                      `json:"os" yaml:"os"`
	OSFamily            string                      `json:"os_family" yaml:"os_family"`
	OSVersion           string                      `json:"os_version" yaml:"os_version"`
	KernelVersion       string                      `json:"kernel_version" yaml:"kernel_version"`
	Interface           string                      `json:"interface" yaml:"interface"`
	Debug               bool                        `json:"debug" yaml:"debug"`
	ListenPort          int                         `json:"listenport" yaml:"listenport"`
	WgPublicListenPort  int                         `json:"wg_public_listen_port" yaml:"wg_public_listen_port"`
	MTU                 int                         `json:"mtu" yaml:"mtu"`
	PublicKey           wgtypes.Key                 `json:"publickey" yaml:"publickey"`
	MacAddress          net.HardwareAddr            `json:"macaddress" yaml:"macaddress"`
	TrafficKeyPublic    datatypes.JSONSlice[byte]   `json:"traffickeypublic" yaml:"traffickeypublic"`
	Nodes               datatypes.JSONSlice[string] `json:"nodes" yaml:"nodes"`
	Interfaces          datatypes.JSONSlice[Iface]  `json:"interfaces" yaml:"interfaces"`
	DefaultInterface    string                      `json:"defaultinterface" yaml:"defaultinterface"`
	EndpointIP          net.IP                      `json:"endpointip" yaml:"endpointip"`
	EndpointIPv6        net.IP                      `json:"endpointipv6" yaml:"endpointipv6"`
	IsDocker            bool                        `json:"isdocker" yaml:"isdocker"`
	IsK8S               bool                        `json:"isk8s" yaml:"isk8s"`
	IsStaticPort        bool                        `json:"isstaticport" yaml:"isstaticport"`
	IsStatic            bool                        `json:"isstatic" yaml:"isstatic"`
	IsDefault           bool                        `json:"isdefault" yaml:"isdefault"`
	DNS                 string                      `json:"dns_status" yaml:"dns_status"`
	NatType             string                      `json:"nat_type,omitempty" yaml:"nat_type,omitempty"`
	TurnEndpoint        *netip.AddrPort             `json:"turn_endpoint,omitempty" yaml:"turn_endpoint,omitempty"`
	PersistentKeepalive time.Duration               `json:"persistentkeepalive" swaggertype:"primitive,integer" format:"int64" yaml:"persistentkeepalive"`
	Location            string                      `json:"location" yaml:"location"` // Format: "lat,lon"
	CountryCode         string                      `json:"country_code" yaml:"country_code"`
	EnableFlowLogs      bool                        `json:"enable_flow_logs" yaml:"enable_flow_logs"`
}

func (h *Host) TableName() string {
	return "hosts_v1"
}
