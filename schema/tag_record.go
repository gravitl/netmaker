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

type TagID string

func (id TagID) String() string { return string(id) }

const (
	OldRemoteAccessTagName = "remote-access-gws"
	GwTagName              = "gateways"
)

type Tag struct {
	ID        TagID     `json:"id"`
	TagName   string    `json:"tag_name"`
	Network   NetworkID `json:"network"`
	ColorCode string    `json:"color_code"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

func (t Tag) GetIDFromName() string {
	return fmt.Sprintf("%s.%s", t.Network, t.TagName)
}

// todo(nm-341): composite primary key
type TagRecord struct {
	Key       string `gorm:"primaryKey"`
	TenantID  string `gorm:"default:'';index"`
	NetworkID string
	Value     datatypes.JSONType[Tag]
}

const tagRecordsTable = "tags"

func (*TagRecord) TableName() string { return tagRecordsTable }

func (r *TagRecord) Get(ctx context.Context) error {
	return db.FromContext(ctx).Where(fmt.Sprintf("key = ? AND %s.tenant_id = ?", tagRecordsTable), r.Key, scope.ID(ctx)).First(r).Error
}

func (r *TagRecord) Upsert(ctx context.Context) error {
	r.TenantID = scope.ID(ctx)
	return db.FromContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(r).Error
}

func (r *TagRecord) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Where(fmt.Sprintf("key = ? AND %s.tenant_id = ?", tagRecordsTable), r.Key, scope.ID(ctx)).Delete(r).Error
}

func (*TagRecord) List(ctx context.Context) ([]TagRecord, error) {
	var records []TagRecord
	query := db.FromContext(ctx).Model(&TagRecord{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", tagRecordsTable), tenantID)(query)
	}
	err := query.Find(&records).Error
	return records, err
}

func (*TagRecord) Count(ctx context.Context) (int, error) {
	var count int64
	query := db.FromContext(ctx).Model(&TagRecord{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", tagRecordsTable), tenantID)(query)
	}
	err := query.Count(&count).Error
	return int(count), err
}
