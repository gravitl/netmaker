package schema

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

const egressTable = "egresses"

type Egress struct {
	ID          string            `gorm:"id,primary_key" json:"id"`
	Name        string            `gorm:"name" json:"name"`
	Network     string            `gorm:"network" json:"network"`
	Description string            `gorm:"description" json:"description"`
	Nodes       datatypes.JSONMap `gorm:"nodes" json:"nodes"`
	Tags        datatypes.JSONMap `gorm:"tags" json:"tags"`
	Range       string            `gorm:"range" json:"range"`
	Nat         bool              `gorm:"nat" json:"nat"`
	IsInetGw    bool              `gorm:"is_internet_gateway" json:"is_internet_gateway"`
	Status      bool              `gorm:"status" json:"status"`
	CreatedBy   string            `gorm:"created_by" json:"created_by"`
	CreatedAt   time.Time         `gorm:"created_at" json:"created_at"`
	UpdatedAt   time.Time         `gorm:"updated_at" json:"updated_at"`
}

func (e *Egress) Table() string {
	return egressTable
}

func (e *Egress) Get(ctx context.Context) error {
	return db.FromContext(ctx).Table(e.Table()).First(&e).Where("id = ?", e.ID).Error
}

func (e *Egress) Update(ctx context.Context) error {
	return db.FromContext(ctx).Table(e.Table()).Where("id = ?", e.ID).Updates(&e).Error
}

func (e *Egress) UpdateNatStatus(ctx context.Context) error {
	return db.FromContext(ctx).Table(e.Table()).Where("id = ?", e.ID).Updates(map[string]any{
		"nat": e.Nat,
	}).Error
}

func (e *Egress) UpdateINetGwStatus(ctx context.Context) error {
	return db.FromContext(ctx).Table(e.Table()).Where("id = ?", e.ID).Updates(map[string]any{
		"is_internet_gateway": e.IsInetGw,
	}).Error
}

func (e *Egress) UpdateEgressStatus(ctx context.Context) error {
	return db.FromContext(ctx).Table(e.Table()).Where("id = ?", e.ID).Updates(map[string]any{
		"status": e.Status,
	}).Error
}

func (e *Egress) Create(ctx context.Context) error {
	return db.FromContext(ctx).Table(e.Table()).Create(&e).Error
}

func (e *Egress) ListByNetwork(ctx context.Context) (egs []Egress, err error) {
	err = db.FromContext(ctx).Table(e.Table()).Where("network = ?", e.Network).Find(&egs).Error
	return
}

func (e *Egress) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Table(e.Table()).Where("id = ?", e.ID).Delete(&e).Error
}
