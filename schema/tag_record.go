package schema

import (
	"context"

	"fmt"
	"time"

	"github.com/gravitl/netmaker/db"

	"gorm.io/datatypes"
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

type TagRecord struct {
	Key       string `gorm:"primaryKey"`
	TenantID  string `gorm:"default:'';index"`
	NetworkID string
	Value     datatypes.JSONType[Tag]
}

func (*TagRecord) TableName() string { return "tags" }

func (r *TagRecord) Get(ctx context.Context) error {
	return db.FromContext(ctx).First(r).Error
}

func (r *TagRecord) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Save(r).Error
}

func (r *TagRecord) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Delete(r).Error
}

func (*TagRecord) List(ctx context.Context) ([]TagRecord, error) {
	var records []TagRecord
	err := db.FromContext(ctx).Find(&records).Error
	return records, err
}

func (*TagRecord) Count(ctx context.Context) (int, error) {
	var count int64
	err := db.FromContext(ctx).Model(&TagRecord{}).Count(&count).Error
	return int(count), err
}
