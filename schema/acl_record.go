package schema

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
	"gorm.io/gorm/clause"
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

const aclRecordsTable = "acls"

func (*AclRecord) TableName() string { return aclRecordsTable }

func (r *AclRecord) Get(ctx context.Context) error {
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

func (r *AclRecord) Upsert(ctx context.Context) error {
	r.TenantID = scope.ID(ctx)
	rec := *r
	rec.Key = TenantScopedKey(r.TenantID, r.Key)
	return db.FromContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&rec).Error
}

func (r *AclRecord) Delete(ctx context.Context) error {
	tenantID := scope.ID(ctx)
	return db.FromContext(ctx).Where("key = ?", TenantScopedKey(tenantID, r.Key)).Delete(&AclRecord{}).Error
}

func (*AclRecord) List(ctx context.Context) ([]AclRecord, error) {
	var records []AclRecord
	query := db.FromContext(ctx).Model(&AclRecord{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", aclRecordsTable), tenantID)(query)
	}
	err := query.Find(&records).Error
	for i := range records {
		records[i].Key = StripTenantKey(records[i].TenantID, records[i].Key)
	}
	return records, err
}

func (*AclRecord) DeleteAll(ctx context.Context) error {
	if tenantID := scope.ID(ctx); tenantID != "" {
		return db.FromContext(ctx).Where(fmt.Sprintf("%s.tenant_id = ?", aclRecordsTable), tenantID).Delete(&AclRecord{}).Error
	}
	return db.FromContext(ctx).Exec(fmt.Sprintf("DELETE FROM %s", aclRecordsTable)).Error
}

func (*AclRecord) Count(ctx context.Context) (int, error) {
	var count int64
	query := db.FromContext(ctx).Model(&AclRecord{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", aclRecordsTable), tenantID)(query)
	}
	err := query.Count(&count).Error
	return int(count), err
}
