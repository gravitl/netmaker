package schema

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/datatypes"
)

// Iface struct for local interfaces of a node
type Iface struct {
	Name          string    `json:"name"`
	Address       net.IPNet `json:"address"`
	AddressString string    `json:"addressString"`
}

type WgKey struct {
	wgtypes.Key
}

func (k WgKey) Value() (driver.Value, error) {
	return k.Key.String(), nil
}

func (k *WgKey) Scan(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected string for WgKey, got %T", value)
	}
	key, err := wgtypes.ParseKey(str)
	if err != nil {
		return err
	}
	k.Key = key
	return nil
}

func (k WgKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.Key)
}

func (k *WgKey) UnmarshalJSON(data []byte) error {
	var key wgtypes.Key
	err := json.Unmarshal(data, &key)
	if err != nil {
		return err
	}

	k.Key = key
	return nil
}

type AddrPort struct {
	netip.AddrPort
}

func (a AddrPort) Value() (driver.Value, error) {
	if !a.IsValid() {
		return nil, nil
	}
	return a.String(), nil
}

func (a *AddrPort) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected string for AddrPort, got %T", value)
	}
	ap, err := netip.ParseAddrPort(str)
	if err != nil {
		return err
	}
	a.AddrPort = ap
	return nil
}

func (a AddrPort) MarshalJSON() ([]byte, error) {
	if !a.IsValid() {
		return json.Marshal(nil)
	}
	return json.Marshal(a.String())
}

func (a *AddrPort) UnmarshalJSON(data []byte) error {
	var s *string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == nil {
		return nil
	}
	ap, err := netip.ParseAddrPort(*s)
	if err != nil {
		return err
	}
	a.AddrPort = ap
	return nil
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
	PublicKey           WgKey                       `json:"publickey" yaml:"publickey"`
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
	TurnEndpoint        *AddrPort                   `json:"turn_endpoint,omitempty" yaml:"turn_endpoint,omitempty"`
	PersistentKeepalive time.Duration               `json:"persistentkeepalive" swaggertype:"primitive,integer" format:"int64" yaml:"persistentkeepalive"`
	Location            string                      `json:"location" yaml:"location"` // Format: "lat,lon"
	CountryCode         string                      `json:"country_code" yaml:"country_code"`
	EnableFlowLogs      bool                        `json:"enable_flow_logs" yaml:"enable_flow_logs"`
}

func (h *Host) TableName() string {
	return "hosts_v1"
}

func (h *Host) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Host{}).Create(h).Error
}

func (h *Host) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Host{}).
		Where("id = ?", h.ID).
		First(h).
		Error
}

func (h *Host) Count(ctx context.Context) (int, error) {
	var count int64
	err := db.FromContext(ctx).Model(&Host{}).Count(&count).Error
	return int(count), err
}

func (h *Host) ListAll(ctx context.Context, options ...dbtypes.Option) ([]Host, error) {
	var hosts []Host
	query := db.FromContext(ctx).Model(&Host{})

	for _, option := range options {
		query = option(query)
	}

	err := query.Find(&hosts).Error
	return hosts, err
}

func (h *Host) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Save(h).Error
}

func (h *Host) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Host{}).
		Where("id = ?", h.ID).
		Delete(h).
		Error
}
