package schema

import "gorm.io/datatypes"

type DNSEntryType string

const (
	DNSEntryType_Node   DNSEntryType = "node"
	DNSEntryType_Custom DNSEntryType = "custom"
)

// DNSEntry - a DNS entry represented as struct
type DNSEntry struct {
	Type     DNSEntryType `json:"type"`
	Address  string       `json:"address" validate:"omitempty,ip"`
	Address6 string       `json:"address6" validate:"omitempty,ip"`
	Name     string       `json:"name" validate:"required,name_unique,min=1,max=192,whitespace"`
	Network  string       `json:"network" validate:"network_exists"`
}

type DNSRecord struct {
	Key       string `gorm:"primaryKey"`
	TenantID  string `gorm:"default:''"`
	NetworkID string
	Value     datatypes.JSONType[DNSEntry]
}

func (*DNSRecord) TableName() string { return "dns" }
