package schema

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
	"gorm.io/gorm/clause"
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
	// SelectedInternetEgressID is the internet egress this config file uses for full-tunnel exit (empty = none).
	SelectedInternetEgressID          string              `json:"selected_internet_egress_id" bson:"selected_internet_egress_id"`
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
	TenantID  string `gorm:"default:''"`
	NetworkID string
	Value     datatypes.JSONType[ExtClient]
}

const extClientRecordsTable = "extclients"

func (*ExtClientRecord) TableName() string { return extClientRecordsTable }

func (r *ExtClientRecord) Get(ctx context.Context) error {
	tenantID := scope.ID(ctx)
	logicalKey := r.Key
	r.Key = TenantScopedKey(tenantID, logicalKey)
	err := db.FromContext(ctx).Where("key = ?", r.Key).First(r).Error
	if err != nil {
		r.Key = logicalKey
		return err
	}
	r.Key = logicalKey
	return nil
}

func (r *ExtClientRecord) Upsert(ctx context.Context) error {
	r.TenantID = scope.ID(ctx)
	rec := *r
	rec.Key = TenantScopedKey(r.TenantID, r.Key)
	return db.FromContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&rec).Error
}

func (r *ExtClientRecord) Delete(ctx context.Context) error {
	tenantID := scope.ID(ctx)
	return db.FromContext(ctx).Where("key = ?", TenantScopedKey(tenantID, r.Key)).Delete(&ExtClientRecord{}).Error
}

func (*ExtClientRecord) List(ctx context.Context) ([]ExtClientRecord, error) {
	var records []ExtClientRecord
	query := db.FromContext(ctx).Model(&ExtClientRecord{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", extClientRecordsTable), tenantID)(query)
	}
	err := query.Find(&records).Error
	for i := range records {
		records[i].Key = StripTenantKey(records[i].TenantID, records[i].Key)
	}
	return records, err
}

func (*ExtClientRecord) Count(ctx context.Context) (int, error) {
	var count int64
	query := db.FromContext(ctx).Model(&ExtClientRecord{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", extClientRecordsTable), tenantID)(query)
	}
	err := query.Count(&count).Error
	return int(count), err
}
