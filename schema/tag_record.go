package schema

import (
	"fmt"
	"time"

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
	Key      string `gorm:"primaryKey"`
	TenantID string `gorm:"default:''"`
	Value    datatypes.JSONType[Tag]
}

func (*TagRecord) TableName() string { return "tags" }
