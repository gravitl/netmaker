package schema

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

// AllowedTrafficDirection - allowed direction of traffic
type AllowedTrafficDirection int

const (
	// TrafficDirectionUni implies traffic is only allowed in one direction (src --> dst)
	TrafficDirectionUni AllowedTrafficDirection = iota
	// TrafficDirectionBi implies traffic is allowed both direction (src <--> dst )
	TrafficDirectionBi
)

// Protocol - allowed protocol
type Protocol string

const (
	ALL  Protocol = "all"
	UDP  Protocol = "udp"
	TCP  Protocol = "tcp"
	ICMP Protocol = "icmp"
)

func (p Protocol) String() string {
	return string(p)
}

type AclPolicyType string

const (
	UserPolicy   AclPolicyType = "user-policy"
	DevicePolicy AclPolicyType = "device-policy"
)

type AclPolicyTag struct {
	ID    AclGroupType `json:"id"`
	Name  string       `json:"name"`
	Value string       `json:"value"`
}

type AclGroupType string

const (
	UserAclID                AclGroupType = "user"
	UserGroupAclID           AclGroupType = "user-group"
	NodeTagID                AclGroupType = "tag"
	NodeID                   AclGroupType = "device"
	EgressRange              AclGroupType = "egress-range"
	EgressID                 AclGroupType = "egress-id"
	NetmakerIPAclID          AclGroupType = "ip"
	NetmakerSubNetRangeAClID AclGroupType = "ipset"
)

func (g AclGroupType) String() string {
	return string(g)
}

type AclPolicy struct {
	TypeID        AclPolicyType
	PrefixTagUser AclGroupType
}

type Acl struct {
	ID               string                  `json:"id"`
	Default          bool                    `json:"default"`
	MetaData         string                  `json:"meta_data"`
	Name             string                  `json:"name"`
	NetworkID        NetworkID               `json:"network_id"`
	RuleType         AclPolicyType           `json:"policy_type"`
	Src              []AclPolicyTag          `json:"src_type"`
	Dst              []AclPolicyTag          `json:"dst_type"`
	Proto            Protocol                `json:"protocol"` // tcp, udp, etc.
	ServiceType      string                  `json:"type"`
	Port             []string                `json:"ports"`
	AllowedDirection AllowedTrafficDirection `json:"allowed_traffic_direction"`
	Enabled          bool                    `json:"enabled"`
	CreatedBy        string                  `json:"created_by"`
	CreatedAt        time.Time               `json:"created_at"`
}

// AclEntry is the GORM model for the legacy "acls" key-value table,
// extended with tenant_id and network_id columns for multi-tenancy.
type AclEntry struct {
	Key       string                  `gorm:"primaryKey;column:key"`
	TenantID  string                  `gorm:"column:tenant_id;default:''"`
	NetworkID string                  `gorm:"column:network_id"`
	Value     datatypes.JSONType[Acl] `gorm:"column:value"`
}

func (*AclEntry) TableName() string { return "acls" }

func (e *AclEntry) Create(ctx context.Context) error {
	return db.FromContext(ctx).Create(e).Error
}

// Save does an upsert — insert or replace on primary key conflict.
func (e *AclEntry) Save(ctx context.Context) error {
	return db.FromContext(ctx).Save(e).Error
}

func (e *AclEntry) Get(ctx context.Context) error {
	return db.FromContext(ctx).Where("key = ?", e.Key).First(e).Error
}

func (e *AclEntry) ListAll(ctx context.Context) ([]AclEntry, error) {
	var entries []AclEntry
	err := db.FromContext(ctx).Find(&entries).Error
	return entries, err
}

func (e *AclEntry) ListByNetwork(ctx context.Context) ([]AclEntry, error) {
	var entries []AclEntry
	err := db.FromContext(ctx).Where("network_id = ?", e.NetworkID).Find(&entries).Error
	return entries, err
}

func (e *AclEntry) Count(ctx context.Context) (int64, error) {
	var count int64
	err := db.FromContext(ctx).Model(&AclEntry{}).Count(&count).Error
	return count, err
}

func (e *AclEntry) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Where("key = ?", e.Key).Delete(&AclEntry{}).Error
}

func (e *AclEntry) DeleteByNetwork(ctx context.Context) error {
	return db.FromContext(ctx).Where("network_id = ?", e.NetworkID).Delete(&AclEntry{}).Error
}
