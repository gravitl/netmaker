package schema

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

// ExtClient - struct for external clients
type ExtClient struct {
	ClientID                          string              `json:"clientid" bson:"clientid"`
	PrivateKey                        string              `json:"privatekey" bson:"privatekey"`
	PublicKey                         string              `json:"publickey" bson:"publickey"`
	Network                           string              `json:"network" bson:"network"`
	DNS                               string              `json:"dns" bson:"dns"`
	Address                           string              `json:"address" bson:"address"`
	Address6                          string              `json:"address6" bson:"address6"`
	ExtraAllowedIPs                   []string            `json:"extraallowedips" bson:"extraallowedips"`
	AllowedIPs                        []string            `json:"allowed_ips"`
	IngressGatewayID                  string              `json:"ingressgatewayid" bson:"ingressgatewayid"`
	IngressGatewayEndpoint            string              `json:"ingressgatewayendpoint" bson:"ingressgatewayendpoint"`
	LastModified                      int64               `json:"lastmodified" bson:"lastmodified" swaggertype:"primitive,integer" format:"int64"`
	Enabled                           bool                `json:"enabled" bson:"enabled"`
	OwnerID                           string              `json:"ownerid" bson:"ownerid"`
	DeniedACLs                        map[string]struct{} `json:"deniednodeacls" bson:"acls,omitempty"`
	RemoteAccessClientID              string              `json:"remote_access_client_id"` // unique ID (MAC address) of RAC machine
	PostUp                            string              `json:"postup" bson:"postup"`
	PostDown                          string              `json:"postdown" bson:"postdown"`
	Tags                              map[TagID]struct{}  `json:"tags"`
	OS                                string              `json:"os"`
	OSFamily                          string              `json:"os_family" yaml:"os_family"`
	OSVersion                         string              `json:"os_version"                      yaml:"os_version"`
	KernelVersion                     string              `json:"kernel_version" yaml:"kernel_version"`
	ClientVersion                     string              `json:"client_version"`
	DeviceID                          string              `json:"device_id"`
	DeviceName                        string              `json:"device_name"`
	PublicEndpoint                    string              `json:"public_endpoint"`
	Country                           string              `json:"country"`
	Location                          string              `json:"location"` //format: lat,long
	PostureChecksViolations           []Violation         `json:"posture_check_violations"`
	PostureCheckVolationSeverityLevel Severity            `json:"posture_check_violation_severity_level"`
	LastEvaluatedAt                   time.Time           `json:"last_evaluated_at"`
	JITExpiresAt                      *time.Time          `json:"jit_expires_at,omitempty" bson:"jit_expires_at,omitempty"` // JIT grant expiry time (nil if JIT not enabled or user is admin)
	Status                            NodeStatus          `json:"status" bson:"status"`
	Mutex                             *sync.Mutex         `json:"-"`
}

// AddressIPNet4 returns the IPv4 address of the ExtClient in IPNet format.
func (extPeer *ExtClient) AddressIPNet4() net.IPNet {
	return net.IPNet{
		IP:   net.ParseIP(extPeer.Address),
		Mask: net.CIDRMask(32, 32),
	}
}

// AddressIPNet6 returns the IPv6 address of the ExtClient in IPNet format.
func (extPeer *ExtClient) AddressIPNet6() net.IPNet {
	return net.IPNet{
		IP:   net.ParseIP(extPeer.Address6),
		Mask: net.CIDRMask(128, 128),
	}
}

// ExtClientEntry is the GORM model for the legacy "extclients" key-value table,
// extended with tenant_id and network_id columns for multi-tenancy.
type ExtClientEntry struct {
	Key       string                        `gorm:"primaryKey;column:key"`
	TenantID  string                        `gorm:"column:tenant_id;default:''"`
	NetworkID string                        `gorm:"column:network_id"`
	Value     datatypes.JSONType[ExtClient] `gorm:"column:value"`
}

func (*ExtClientEntry) TableName() string { return "extclients" }

func (e *ExtClientEntry) Create(ctx context.Context) error {
	return db.FromContext(ctx).Create(e).Error
}

// Save does an upsert — insert or replace on primary key conflict.
func (e *ExtClientEntry) Save(ctx context.Context) error {
	return db.FromContext(ctx).Save(e).Error
}

func (e *ExtClientEntry) Get(ctx context.Context) error {
	return db.FromContext(ctx).Where("key = ?", e.Key).First(e).Error
}

func (e *ExtClientEntry) ListAll(ctx context.Context) ([]ExtClientEntry, error) {
	var entries []ExtClientEntry
	err := db.FromContext(ctx).Find(&entries).Error
	return entries, err
}

func (e *ExtClientEntry) ListByNetwork(ctx context.Context) ([]ExtClientEntry, error) {
	var entries []ExtClientEntry
	err := db.FromContext(ctx).Where("network_id = ?", e.NetworkID).Find(&entries).Error
	return entries, err
}

func (e *ExtClientEntry) Count(ctx context.Context) (int64, error) {
	var count int64
	err := db.FromContext(ctx).Model(&ExtClientEntry{}).Count(&count).Error
	return count, err
}

func (e *ExtClientEntry) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Where("key = ?", e.Key).Delete(&ExtClientEntry{}).Error
}

func (e *ExtClientEntry) DeleteByNetwork(ctx context.Context) error {
	return db.FromContext(ctx).Where("network_id = ?", e.NetworkID).Delete(&ExtClientEntry{}).Error
}
