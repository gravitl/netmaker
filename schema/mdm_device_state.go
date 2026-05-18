package schema

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
)

const deviceMDMStateTable = "device_mdm_state_v1"

// MatchedBy* identify how a host was matched to an MDM-managed device record.
const (
	MDMMatchEntraDeviceID = "entra_device_id"
	MDMMatchSerialNumber  = "serial_number"
	MDMMatchHardwareUUID  = "hardware_uuid"
	MDMMatchHostnameEmail = "hostname_email"
	MDMMatchHostname      = "hostname"
)

// DeviceMDMState is the per-host snapshot of an MDM provider's view of a device.
// Composite PK (HostID, Provider) allows a host to carry historical state for
// multiple providers (e.g. after switching MDMs).
type DeviceMDMState struct {
	HostID       string    `gorm:"primaryKey;column:host_id" json:"host_id"`
	Provider     string    `gorm:"primaryKey;column:provider" json:"provider"`
	MDMDeviceID  string    `gorm:"column:mdm_device_id" json:"mdm_device_id"`
	Enrolled     bool      `gorm:"column:enrolled" json:"enrolled"`
	Compliant    bool      `gorm:"column:compliant" json:"compliant"`
	MatchedBy    string    `gorm:"column:matched_by" json:"matched_by"`
	LastSyncedAt time.Time `gorm:"column:last_synced_at" json:"last_synced_at"`
	LastSeenAt   time.Time `gorm:"column:last_seen_at" json:"last_seen_at"`
}

func (s *DeviceMDMState) TableName() string {
	return deviceMDMStateTable
}

// Get loads the row identified by (HostID, Provider).
func (s *DeviceMDMState) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&DeviceMDMState{}).
		Where("host_id = ? AND provider = ?", s.HostID, s.Provider).
		First(s).Error
}

// Upsert inserts the row or updates the existing one keyed by (HostID, Provider).
func (s *DeviceMDMState) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Save(s).Error
}

// Delete removes the row for (HostID, Provider).
func (s *DeviceMDMState) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&DeviceMDMState{}).
		Where("host_id = ? AND provider = ?", s.HostID, s.Provider).
		Delete(&DeviceMDMState{}).Error
}

// DeleteByHostID removes all MDM state rows for a given host (used when a host is deleted).
func (s *DeviceMDMState) DeleteByHostID(ctx context.Context) error {
	return db.FromContext(ctx).Model(&DeviceMDMState{}).
		Where("host_id = ?", s.HostID).
		Delete(&DeviceMDMState{}).Error
}

// ListByHost returns all provider states for the host in s.HostID.
func (s *DeviceMDMState) ListByHost(ctx context.Context) ([]DeviceMDMState, error) {
	var out []DeviceMDMState
	err := db.FromContext(ctx).Model(&DeviceMDMState{}).
		Where("host_id = ?", s.HostID).
		Find(&out).Error
	return out, err
}

// ListByProvider returns all host states for the provider in s.Provider.
func (s *DeviceMDMState) ListByProvider(ctx context.Context) ([]DeviceMDMState, error) {
	var out []DeviceMDMState
	err := db.FromContext(ctx).Model(&DeviceMDMState{}).
		Where("provider = ?", s.Provider).
		Find(&out).Error
	return out, err
}

// ListAll returns every MDM state row.
func (s *DeviceMDMState) ListAll(ctx context.Context) ([]DeviceMDMState, error) {
	var out []DeviceMDMState
	err := db.FromContext(ctx).Model(&DeviceMDMState{}).Find(&out).Error
	return out, err
}
