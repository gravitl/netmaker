package types

import (
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

type ExtClientRecord struct {
	Key       string `gorm:"primaryKey"`
	TenantID  string `gorm:"default:''"`
	NetworkID string
	Value     datatypes.JSONType[schema.ExtClient]
}

func (*ExtClientRecord) TableName() string { return "extclients" }
