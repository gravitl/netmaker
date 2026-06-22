package schema

import (
	"time"

	"gorm.io/datatypes"
)

// CacheValue - the cache object for a network
type CacheValue struct {
	Network    string    `json:"network,omitempty"`
	Value      string    `json:"value"`
	Host       Host      `json:"host"`
	Pass       string    `json:"pass,omitempty"`
	User       string    `json:"user,omitempty"`
	ALL        bool      `json:"all,omitempty"`
	Expiration time.Time `json:"expiration"`
}

type CacheRecord struct {
	Key      string `gorm:"primaryKey"`
	TenantID string `gorm:"default:''"`
	Value    datatypes.JSONType[CacheValue]
}

func (*CacheRecord) TableName() string { return "cache" }
