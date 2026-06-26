package schema

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

const deviceEDRStateTable = "device_edr_state_v1"

// EDR match identifiers (how a host was matched to an EDR endpoint record).
const (
	EDRMatchEntraDeviceID = "entra_device_id"
	EDRMatchSerialNumber  = "serial_number"
	EDRMatchHostname      = "hostname"
)

// DeviceEDRState is the per-host snapshot of an EDR provider's view of an endpoint.
type DeviceEDRState struct {
	HostID       string         `gorm:"primaryKey;column:host_id" json:"host_id"`
	Provider     string         `gorm:"primaryKey;column:provider" json:"provider"`
	EDRDeviceID  string         `gorm:"column:edr_device_id" json:"edr_device_id"`
	MatchedBy    string         `gorm:"column:matched_by" json:"matched_by"`
	AgentInstalled bool         `gorm:"column:agent_installed" json:"agent_installed"`
	AgentHealthy   bool         `gorm:"column:agent_healthy" json:"agent_healthy"`
	RiskLevel      string         `gorm:"column:risk_level" json:"risk_level"`
	ThreatCount    int            `gorm:"column:threat_count" json:"threat_count"`
	ActiveThreats  bool           `gorm:"column:active_threats" json:"active_threats"`
	Isolated       bool           `gorm:"column:isolated" json:"isolated"`
	Contained      bool           `gorm:"column:contained" json:"contained"`
	LastSeenAt     time.Time      `gorm:"column:last_seen_at" json:"last_seen_at"`
	LastSyncedAt   time.Time      `gorm:"column:last_synced_at" json:"last_synced_at"`
	LastError      string         `gorm:"column:last_error" json:"last_error,omitempty"`
	RawVendorData  datatypes.JSON `gorm:"column:raw_vendor_data" json:"raw_vendor_data,omitempty"`
}

func (s *DeviceEDRState) TableName() string {
	return deviceEDRStateTable
}

func (s *DeviceEDRState) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&DeviceEDRState{}).
		Where("host_id = ? AND provider = ?", s.HostID, s.Provider).
		First(s).Error
}

func (s *DeviceEDRState) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Save(s).Error
}

func (s *DeviceEDRState) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&DeviceEDRState{}).
		Where("host_id = ? AND provider = ?", s.HostID, s.Provider).
		Delete(&DeviceEDRState{}).Error
}

func (s *DeviceEDRState) DeleteByHostID(ctx context.Context) error {
	return db.FromContext(ctx).Model(&DeviceEDRState{}).
		Where("host_id = ?", s.HostID).
		Delete(&DeviceEDRState{}).Error
}

func (s *DeviceEDRState) ListByHost(ctx context.Context) ([]DeviceEDRState, error) {
	var out []DeviceEDRState
	err := db.FromContext(ctx).Model(&DeviceEDRState{}).
		Where("host_id = ?", s.HostID).
		Find(&out).Error
	return out, err
}

func (s *DeviceEDRState) ListByProvider(ctx context.Context) ([]DeviceEDRState, error) {
	var out []DeviceEDRState
	err := db.FromContext(ctx).Model(&DeviceEDRState{}).
		Where("provider = ?", s.Provider).
		Find(&out).Error
	return out, err
}

func (s *DeviceEDRState) ListAll(ctx context.Context) ([]DeviceEDRState, error) {
	var out []DeviceEDRState
	err := db.FromContext(ctx).Model(&DeviceEDRState{}).Find(&out).Error
	return out, err
}
