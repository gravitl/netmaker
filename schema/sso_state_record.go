package schema

import (
	"time"

	"gorm.io/datatypes"
)

const DefaultSsoStateDuration = time.Minute * 5

// SsoState - holds SSO sign-in session data
type SsoState struct {
	AppName    string    `json:"app_name"`
	Value      string    `json:"value"`
	Expiration time.Time `json:"expiration"`
}

func (s *SsoState) IsExpired() bool { return time.Now().After(s.Expiration) }

type SsoStateRecord struct {
	Key      string `gorm:"primaryKey"`
	TenantID string `gorm:"default:''"`
	Value    datatypes.JSONType[SsoState]
}

func (*SsoStateRecord) TableName() string { return "ssostatecache" }
