package models

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

const egressTable = "egresses"

type EgressReq struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Network     string         `json:"network"`
	Description string         `json:"description"`
	Nodes       map[string]int `json:"nodes"`
	Tags        []string       `json:"tags"`
	Range       string         `json:"range"`
	Nat         bool           `json:"nat"`
}

type Egress struct {
	ID          string            `gorm:"id,primary_key" json:"id"`
	Name        string            `gorm:"name" json:"name"`
	Network     string            `gorm:"network" json:"network"`
	Description string            `gorm:"description" json:"description"`
	Nodes       datatypes.JSONMap `gorm:"nodes" json:"nodes"`
	Tags        datatypes.JSONMap `gorm:"tags" json:"tags"`
	Range       string            `gorm:"range" json:"range"`
	Nat         bool              `gorm:"nat" json:"nat"`
	CreatedBy   string            `gorm:"created_by" json:"created_by"`
	CreatedAt   time.Time         `gorm:"created_at" json:"created_at"`
	UpdatedAt   time.Time         `gorm:"updated_at" json:"updated_at"`
}

func (e *Egress) Table() string {
	return egressTable
}

func (e *Egress) Get() error {
	return db.FromContext(context.TODO()).Table(e.Table()).First(&e).Where("id = ?", e.ID).Error
}

func (e *Egress) Update() error {
	return db.FromContext(context.TODO()).Table(e.Table()).Where("id = ?", e.ID).Updates(&e).Error
}

func (e *Egress) UpdateNatStatus() error {
	return db.FromContext(context.TODO()).Table(e.Table()).Where("id = ?", e.ID).Updates(map[string]any{
		"nat": e.Nat,
	}).Error
}

func (e *Egress) Create() error {
	return db.FromContext(context.TODO()).Table(e.Table()).Create(&e).Error
}

func (e *Egress) ListByNetwork() (egs []Egress, err error) {
	err = db.FromContext(context.TODO()).Table(e.Table()).Where("network = ?", e.Network).Find(&egs).Error
	return
}

func (e *Egress) Delete() error {
	return db.FromContext(context.TODO()).Table(e.Table()).Where("id = ?", e.ID).Delete(&e).Error
}
