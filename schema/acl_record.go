package schema

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
	"gorm.io/gorm"
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
	ID        string `gorm:"primaryKey"`
	TenantID  string `gorm:"default:'';uniqueIndex:udx_acl_tenant_slug"`
	Slug      string `gorm:"not null;uniqueIndex:udx_acl_tenant_slug"`
	NetworkID string
	Value     datatypes.JSONType[Acl]
}

const aclRecordsTable = "acls_v1"

var ErrAclIdentifiersNotProvided = errors.New("acl identifiers not provided")

func (*AclRecord) TableName() string { return aclRecordsTable }

// baseIdentifierQuery scopes the query to the acl identified by its ID, or
// by its slug within the tenant in the context.
func (r *AclRecord) baseIdentifierQuery(ctx context.Context) (*gorm.DB, error) {
	tenantID := scope.ID(ctx)
	query := db.FromContext(ctx).Model(&AclRecord{})
	if r.ID != "" {
		query = query.Where(fmt.Sprintf("%s.id = ?", aclRecordsTable), r.ID)
		if tenantID != "" {
			query = query.Where(fmt.Sprintf("%s.tenant_id = ?", aclRecordsTable), tenantID)
		}
		return query, nil
	}

	if r.Slug == "" {
		return nil, ErrAclIdentifiersNotProvided
	}

	return query.Where(fmt.Sprintf("%s.tenant_id = ? AND %s.slug = ?", aclRecordsTable, aclRecordsTable), tenantID, r.Slug), nil
}

// Get fetches the acl by its ID, or by its slug.
func (r *AclRecord) Get(ctx context.Context) error {
	query, err := r.baseIdentifierQuery(ctx)
	if err != nil {
		return err
	}

	var record AclRecord
	err = query.First(&record).Error
	if err != nil {
		return err
	}

	*r = record
	return nil
}

// Upsert updates the acl identified by its ID, or by its slug, creating it
// if it does not exist.
func (r *AclRecord) Upsert(ctx context.Context) error {
	r.TenantID = scope.ID(ctx)
	rec := *r
	onConflict := clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "slug"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}
	if rec.ID != "" {
		onConflict = clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"slug", "network_id", "value"}),
		}
	} else {
		rec.ID = uuid.NewString()
	}
	return db.FromContext(ctx).Clauses(onConflict).Create(&rec).Error
}

// Delete deletes the acl identified by its ID, or by its slug.
func (r *AclRecord) Delete(ctx context.Context) error {
	query, err := r.baseIdentifierQuery(ctx)
	if err != nil {
		return err
	}

	return query.Delete(&AclRecord{}).Error
}

func (*AclRecord) List(ctx context.Context) ([]AclRecord, error) {
	var records []AclRecord
	query := db.FromContext(ctx).Model(&AclRecord{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", aclRecordsTable), tenantID)(query)
	}
	err := query.Find(&records).Error
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
