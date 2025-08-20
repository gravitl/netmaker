package schema

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

type Nameserver struct {
	ID          string                      `gorm:"primaryKey" json:"id"`
	Name        string                      `gorm:"name" json:"name"`
	Network     string                      `gorm:"network" json:"network"`
	Description string                      `gorm:"description" json:"description"`
	Servers     datatypes.JSONSlice[string] `gorm:"servers" json:"servers"`
	MatchDomain string                      `gorm:"match_domain" json:"match_domain"`
	Tags        datatypes.JSONMap           `gorm:"tags" json:"tags"`
	Status      bool                        `gorm:"status" json:"status"`
	CreatedBy   string                      `gorm:"created_by" json:"created_by"`
	CreatedAt   time.Time                   `gorm:"created_at" json:"created_at"`
	UpdatedAt   time.Time                   `gorm:"updated_at" json:"updated_at"`
}

func (ns *Nameserver) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Nameserver{}).First(&ns).Where("id = ?", ns.ID).Error
}

func (ns *Nameserver) Update(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Nameserver{}).Where("id = ?", ns.ID).Updates(&ns).Error
}

func (ns *Nameserver) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Nameserver{}).Create(&ns).Error
}

func (ns *Nameserver) ListByNetwork(ctx context.Context) (dnsli []Nameserver, err error) {
	err = db.FromContext(ctx).Model(&Nameserver{}).Where("network_id = ?", ns.Network).Find(&dnsli).Error
	return
}

func (ns *Nameserver) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Nameserver{}).Where("id = ?", ns.ID).Delete(&ns).Error
}

func (ns *Nameserver) UpdateStatus(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Nameserver{}).Where("id = ?", ns.ID).Updates(map[string]any{
		"status": ns.Status,
	}).Error
}
