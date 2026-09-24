package types

import (
	"github.com/gravitl/netmaker/models"
	"gorm.io/datatypes"
)

type ExtClientRecord struct {
	Key       string `gorm:"primaryKey"`
	TenantID  string `gorm:"default:''"`
	NetworkID string
	Value     datatypes.JSONType[models.ExtClient]
}

func (*ExtClientRecord) TableName() string { return "extclients" }
