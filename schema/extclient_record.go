package schema

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

// Violation - posture check violation data
type Violation struct {
	CheckID   string   `json:"check_id"`
	Name      string   `json:"name"`
	Attribute string   `json:"attribute"`
	Message   string   `json:"message"`
	Severity  Severity `json:"severity"`
}

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
	RemoteAccessClientID              string              `json:"remote_access_client_id"`
	PostUp                            string              `json:"postup" bson:"postup"`
	PostDown                          string              `json:"postdown" bson:"postdown"`
	Tags                              map[TagID]struct{}  `json:"tags"`
	OS                                string              `json:"os"`
	OSFamily                          string              `json:"os_family" yaml:"os_family"`
	OSVersion                         string              `json:"os_version" yaml:"os_version"`
	KernelVersion                     string              `json:"kernel_version" yaml:"kernel_version"`
	ClientVersion                     string              `json:"client_version"`
	DeviceID                          string              `json:"device_id"`
	DeviceName                        string              `json:"device_name"`
	PublicEndpoint                    string              `json:"public_endpoint"`
	Country                           string              `json:"country"`
	Location                          string              `json:"location"`
	PostureChecksViolations           []Violation         `json:"posture_check_violations"`
	PostureCheckVolationSeverityLevel Severity            `json:"posture_check_violation_severity_level"`
	LastEvaluatedAt                   time.Time           `json:"last_evaluated_at"`
	JITExpiresAt                      *time.Time          `json:"jit_expires_at,omitempty" bson:"jit_expires_at,omitempty"`
	Status                            NodeStatus          `json:"status" bson:"status"`
	Mutex                             *sync.Mutex         `json:"-"`
}

func (extPeer *ExtClient) AddressIPNet4() net.IPNet {
	return net.IPNet{IP: net.ParseIP(extPeer.Address), Mask: net.CIDRMask(32, 32)}
}

func (extPeer *ExtClient) AddressIPNet6() net.IPNet {
	return net.IPNet{IP: net.ParseIP(extPeer.Address6), Mask: net.CIDRMask(128, 128)}
}

type ExtClientRecord struct {
	Key       string `gorm:"primaryKey"`
	TenantID  string `gorm:"default:'';index"`
	NetworkID string
	Value     datatypes.JSONType[ExtClient]
}

func (*ExtClientRecord) TableName() string { return "extclients" }

func (r *ExtClientRecord) Get(ctx context.Context) error {
	return db.FromContext(ctx).First(r).Error
}

func (r *ExtClientRecord) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Save(r).Error
}

func (r *ExtClientRecord) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Delete(r).Error
}

func (*ExtClientRecord) List(ctx context.Context) ([]ExtClientRecord, error) {
	var records []ExtClientRecord
	err := db.FromContext(ctx).Find(&records).Error
	return records, err
}

func (*ExtClientRecord) Count(ctx context.Context) (int, error) {
	var count int64
	err := db.FromContext(ctx).Model(&ExtClientRecord{}).Count(&count).Error
	return int(count), err
}
