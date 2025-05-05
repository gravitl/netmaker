package schema

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"gorm.io/datatypes"
)

type Event struct {
	ID          string           `gorm:"primaryKey" json:"id"`
	Action      models.Action    `gorm:"action" json:"action"`
	Source      datatypes.JSON   `gorm:"source" json:"source"`
	Origin      models.Origin    `gorm:"origin" json:"origin"`
	Target      datatypes.JSON   `gorm:"target" json:"target"`
	NetworkID   models.NetworkID `gorm:"network_id" json:"network_id"`
	TriggeredBy string           `gorm:"triggered_by" json:"triggered_by"`
	TimeStamp   time.Time        `gorm:"time_stamp" json:"time_stamp"`
}

func (a *Event) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Event{}).First(&a).Where("id = ?", a.ID).Error
}

func (a *Event) Update(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Event{}).Where("id = ?", a.ID).Updates(&a).Error
}

func (a *Event) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Event{}).Create(&a).Error
}

func (a *Event) ListByNetwork(ctx context.Context) (ats []Event, err error) {
	err = db.FromContext(ctx).Model(&Event{}).Where("network_id = ?", a.NetworkID).Order("time_stamp DESC").Find(&ats).Error
	return
}

func (a *Event) ListByUser(ctx context.Context) (ats []Event, err error) {
	err = db.FromContext(ctx).Model(&Event{}).Where("triggered_by = ?", a.TriggeredBy).Order("time_stamp DESC").Find(&ats).Error
	return
}

func (a *Event) List(ctx context.Context) (ats []Event, err error) {
	err = db.FromContext(ctx).Model(&Event{}).Order("time_stamp DESC").Find(&ats).Error
	return
}
