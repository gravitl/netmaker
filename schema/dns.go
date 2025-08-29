package schema

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

type Nameserver struct {
	ID           string                      `gorm:"primaryKey" json:"id"`
	Name         string                      `gorm:"name" json:"name"`
	NetworkID    string                      `gorm:"network_id" json:"network_id"`
	Description  string                      `gorm:"description" json:"description"`
	Servers      datatypes.JSONSlice[string] `gorm:"servers" json:"servers"`
	MatchAll     bool                        `gorm:"match_all" json:"match_all"`
	MatchDomains datatypes.JSONSlice[string] `gorm:"match_domains" json:"match_domains"`
	Tags         datatypes.JSONMap           `gorm:"tags" json:"tags"`
	Nodes        datatypes.JSONMap           `gorm:"nodes" json:"nodes"`
	Status       bool                        `gorm:"status" json:"status"`
	CreatedBy    string                      `gorm:"created_by" json:"created_by"`
	CreatedAt    time.Time                   `gorm:"created_at" json:"created_at"`
	UpdatedAt    time.Time                   `gorm:"updated_at" json:"updated_at"`
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
	err = db.FromContext(ctx).Model(&Nameserver{}).Where("network_id = ?", ns.NetworkID).Find(&dnsli).Error
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

func (ns *Nameserver) UpdateMatchAll(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Nameserver{}).Where("id = ?", ns.ID).Updates(map[string]any{
		"match_all": ns.MatchAll,
	}).Error
}
