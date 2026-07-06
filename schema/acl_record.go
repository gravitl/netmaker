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
	TrafficDirectionUni AllowedTrafficDirection = iota
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

type AclPolicyType string

const (
	UserPolicy   AclPolicyType = "user-policy"
	DevicePolicy AclPolicyType = "device-policy"
)

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

func (g AclGroupType) String() string { return string(g) }

func (p Protocol) String() string { return string(p) }

type AclPolicyTag struct {
	ID    AclGroupType `json:"id"`
	Name  string       `json:"name"`
	Value string       `json:"value"`
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
	Proto            Protocol                `json:"protocol"`
	ServiceType      string                  `json:"type"`
	Port             []string                `json:"ports"`
	AllowedDirection AllowedTrafficDirection `json:"allowed_traffic_direction"`
	Enabled          bool                    `json:"enabled"`
	CreatedBy        string                  `json:"created_by"`
	CreatedAt        time.Time               `json:"created_at"`
}

type AclRecord struct {
	Key       string `gorm:"primaryKey"`
	TenantID  string `gorm:"default:'';index"`
	NetworkID string
	Value     datatypes.JSONType[Acl]
}

func (*AclRecord) TableName() string { return "acls" }

func (r *AclRecord) Get(ctx context.Context) error {
	return db.FromContext(ctx).First(r).Error
}

func (r *AclRecord) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Save(r).Error
}

func (r *AclRecord) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Delete(r).Error
}

func (*AclRecord) List(ctx context.Context) ([]AclRecord, error) {
	var records []AclRecord
	err := db.FromContext(ctx).Find(&records).Error
	return records, err
}

func (*AclRecord) Count(ctx context.Context) (int, error) {
	var count int64
	err := db.FromContext(ctx).Model(&AclRecord{}).Count(&count).Error
	return int(count), err
}
