package models

import (
	"context"
	"net"
	"time"

	"github.com/gravitl/netmaker/db"
)

const egressTable = "egress"

type Egress struct {
	ID          string    `gorm:"id,primary_key" json:"id"`
	Name        string    `gorm:"name" json:"name"`
	Network     string    `gorm:"network" json:"network"`
	Description string    `gorm:"description" json:"description"`
	Nodes       []string  `gorm:"nodes" json:"nodes"`
	Tags        []TagID   `gorm:"tags" json:"tags"`
	Range       net.IPNet `gorm:"range" json:"range"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
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
