package schema

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

type PendingHost struct {
	ID            string         `gorm:"id" json:"id"`
	HostID        string         `gorm:"host_id" json:"host_id"`
	Hostname      string         `gorm:"host_name" json:"host_name"`
	Network       string         `gorm:"network" json:"network"`
	PublicKey     string         `gorm:"public_key" json:"public_key"`
	EnrollmentKey datatypes.JSON `gorm:"enrollment_key_id" json:"enrollment_key_id"`
	OS            string         `gorm:"os" json:"os"`
	Version       string         `gorm:"version" json:"version"`
	Location      string         `gorm:"location" json:"location"` // Format: "lat,lon"
	RequestedAt   time.Time      `gorm:"requested_at" json:"requested_at"`
}

func (p *PendingHost) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&PendingHost{}).First(&p).Where("id = ?", p.ID).Error
}

func (p *PendingHost) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&PendingHost{}).Create(&p).Error
}

func (p *PendingHost) List(ctx context.Context) (pendingHosts []PendingHost, err error) {
	err = db.FromContext(ctx).Model(&PendingHost{}).Find(&pendingHosts).Error
	return
}

func (p *PendingHost) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&PendingHost{}).Where("id = ?", p.ID).Delete(&p).Error
}
func (p *PendingHost) CheckIfPendingHostExists(ctx context.Context) error {
	return db.FromContext(ctx).Model(&PendingHost{}).Where("host_id = ? AND network = ?", p.HostID, p.Network).First(&p).Error
}

func (p *PendingHost) DeleteAllPendingHosts(ctx context.Context) error {
	return db.FromContext(ctx).Model(&PendingHost{}).Where("host_id = ?", p.HostID).Delete(&p).Error
}
