package types

import (
	"time"

	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

// ExtClientRecord is the key-value row extclients were stored as before
// v1.8.0. Used only during the v1.8.0 migration to populate extclients_v1.
type ExtClientRecord struct {
	Key       string `gorm:"primaryKey"`
	TenantID  string `gorm:"default:''"`
	NetworkID string
	Value     datatypes.JSONType[ExtClient]
}

func (*ExtClientRecord) TableName() string {
	return "extclients"
}

// ExtClient is the json value of an ExtClientRecord.
type ExtClient struct {
	ClientID                          string                    `json:"clientid"`
	PrivateKey                        string                    `json:"privatekey"`
	PublicKey                         string                    `json:"publickey"`
	Network                           string                    `json:"network"`
	DNS                               string                    `json:"dns"`
	Address                           string                    `json:"address"`
	Address6                          string                    `json:"address6"`
	ExtraAllowedIPs                   []string                  `json:"extraallowedips"`
	AllowedIPs                        []string                  `json:"allowed_ips"`
	IngressGatewayID                  string                    `json:"ingressgatewayid"`
	IngressGatewayEndpoint            string                    `json:"ingressgatewayendpoint"`
	SelectedInternetEgressID          string                    `json:"selected_internet_egress_id"`
	LastModified                      int64                     `json:"lastmodified"`
	Enabled                           bool                      `json:"enabled"`
	OwnerID                           string                    `json:"ownerid"`
	DeniedACLs                        map[string]struct{}       `json:"deniednodeacls"`
	RemoteAccessClientID              string                    `json:"remote_access_client_id"`
	PostUp                            string                    `json:"postup"`
	PostDown                          string                    `json:"postdown"`
	Tags                              map[schema.TagID]struct{} `json:"tags"`
	OS                                string                    `json:"os"`
	OSFamily                          string                    `json:"os_family"`
	OSVersion                         string                    `json:"os_version"`
	KernelVersion                     string                    `json:"kernel_version"`
	ClientVersion                     string                    `json:"client_version"`
	DeviceID                          string                    `json:"device_id"`
	DeviceName                        string                    `json:"device_name"`
	PublicEndpoint                    string                    `json:"public_endpoint"`
	Country                           string                    `json:"country"`
	Location                          string                    `json:"location"`
	PostureChecksViolations           []Violation               `json:"posture_check_violations"`
	PostureCheckVolationSeverityLevel schema.Severity           `json:"posture_check_violation_severity_level"`
	LastEvaluatedAt                   time.Time                 `json:"last_evaluated_at"`
	JITExpiresAt                      *time.Time                `json:"jit_expires_at,omitempty"`
	Status                            schema.NodeStatus         `json:"status"`
}

// Violation is a posture check violation stored in an ExtClient.
type Violation struct {
	CheckID   string          `json:"check_id"`
	Name      string          `json:"name"`
	Attribute string          `json:"attribute"`
	Message   string          `json:"message"`
	Severity  schema.Severity `json:"severity"`
}

// AclRecord is the key-value row acls were stored as before v1.8.0, keyed
// by <tenant id>::<acl id>. Used only during the v1.8.0 migration to populate
// acls_v1.
type AclRecord struct {
	Key       string `gorm:"primaryKey"`
	TenantID  string `gorm:"default:''"`
	NetworkID string
	Value     datatypes.JSONType[schema.Acl]
}

func (*AclRecord) TableName() string {
	return "acls"
}

// TagRecord is the key-value row tags were stored as before v1.8.0, keyed by
// <tenant id>::<tag id>. Used only during the v1.8.0 migration to populate
// tags_v1.
type TagRecord struct {
	Key       string `gorm:"primaryKey"`
	TenantID  string `gorm:"default:''"`
	NetworkID string
	Value     datatypes.JSONType[Tag]
}

func (*TagRecord) TableName() string {
	return "tags"
}

// Tag is the json value of a TagRecord.
type Tag struct {
	ID        schema.TagID     `json:"id"`
	TagName   string           `json:"tag_name"`
	Network   schema.NetworkID `json:"network"`
	ColorCode string           `json:"color_code"`
	CreatedBy string           `json:"created_by"`
	CreatedAt time.Time        `json:"created_at"`
}

// DNSRecord is the key-value row custom dns entries were stored as before
// v1.8.0, keyed by <tenant id>::<name>###<network>. Used only during the
// v1.8.0 migration to populate dns_v1.
type DNSRecord struct {
	Key       string `gorm:"primaryKey"`
	TenantID  string `gorm:"default:''"`
	NetworkID string
	Value     datatypes.JSONType[DNSEntry]
}

func (*DNSRecord) TableName() string {
	return "dns"
}

// DNSEntry is the json value of a DNSRecord.
type DNSEntry struct {
	Type     schema.DNSEntryType `json:"type"`
	Address  string              `json:"address"`
	Address6 string              `json:"address6"`
	Name     string              `json:"name"`
	Network  string              `json:"network"`
}
