package schema

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
)

const pendingHostsTable = "pending_hosts"

type PendingHost struct {
	ID            string         `gorm:"id" json:"id"`
	TenantID      string         `gorm:"default:'';index" json:"tenant_id"`
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

func (p *PendingHost) TableName() string {
	return pendingHostsTable
}

func (p *PendingHost) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&PendingHost{}).First(&p).Where("id = ?", p.ID).Error
}

func (p *PendingHost) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&PendingHost{}).Create(&p).Error
}

func (p *PendingHost) List(ctx context.Context) (pendingHosts []PendingHost, err error) {
	query := db.FromContext(ctx).Model(&PendingHost{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", pendingHostsTable), tenantID)(query)
	}
	err = query.Find(&pendingHosts).Error
	return
}

func (p *PendingHost) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&PendingHost{}).Where("id = ?", p.ID).Delete(&p).Error
}
func (p *PendingHost) CheckIfPendingHostExists(ctx context.Context) error {
	return db.FromContext(ctx).Model(&PendingHost{}).Where("host_id = ? AND network = ?", p.HostID, p.Network).First(&p).Error
}

func (p *PendingHost) DeleteAllPendingHosts(ctx context.Context) error {
	query := db.FromContext(ctx).Model(&PendingHost{}).Where("host_id = ?", p.HostID)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", pendingHostsTable), tenantID)(query)
	}
	return query.Delete(&p).Error
}
