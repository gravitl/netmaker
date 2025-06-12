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
	Diff        datatypes.JSON   `gorm:"diff" json:"diff"`
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

func (a *Event) ListByNetwork(ctx context.Context, from, to time.Time) (ats []Event, err error) {
	if !from.IsZero() && !to.IsZero() {
		// "created_at BETWEEN ? AND ?
		err = db.FromContext(ctx).Model(&Event{}).Where("network_id = ? AND time_stamp BETWEEN ? AND ?",
			a.NetworkID, from, to).Order("time_stamp DESC").Find(&ats).Error
		return
	}
	err = db.FromContext(ctx).Model(&Event{}).Where("network_id = ?", a.NetworkID).Order("time_stamp DESC").Find(&ats).Error

	return
}

func (a *Event) ListByUser(ctx context.Context, from, to time.Time) (ats []Event, err error) {
	if !from.IsZero() && !to.IsZero() {
		err = db.FromContext(ctx).Model(&Event{}).Where("triggered_by = ? AND time_stamp BETWEEN ? AND ?",
			a.TriggeredBy, from, to).Order("time_stamp DESC").Find(&ats).Error
		return
	}
	err = db.FromContext(ctx).Model(&Event{}).Where("triggered_by = ?", a.TriggeredBy).Order("time_stamp DESC").Find(&ats).Error
	return
}

func (a *Event) ListByUserAndNetwork(ctx context.Context, from, to time.Time) (ats []Event, err error) {
	if !from.IsZero() && !to.IsZero() {
		err = db.FromContext(ctx).Model(&Event{}).Where("network_id = ? AND triggered_by = ? AND time_stamp BETWEEN ? AND ?",
			a.NetworkID, a.TriggeredBy, from, to).Order("time_stamp DESC").Find(&ats).Error
		return
	}
	err = db.FromContext(ctx).Model(&Event{}).Where("network_id = ? AND triggered_by = ?",
		a.NetworkID, a.TriggeredBy).Order("time_stamp DESC").Find(&ats).Error
	return
}

func (a *Event) List(ctx context.Context, from, to time.Time) (ats []Event, err error) {
	if !from.IsZero() && !to.IsZero() {
		err = db.FromContext(ctx).Model(&Event{}).Where("time_stamp BETWEEN ? AND ?", from, to).Order("time_stamp DESC").Find(&ats).Error
		return
	}
	err = db.FromContext(ctx).Model(&Event{}).Order("time_stamp DESC").Find(&ats).Error
	return
}

func (a *Event) DeleteOldEvents(ctx context.Context, retentionDays int) error {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	return db.FromContext(ctx).Model(&Event{}).Where("created_at < ?", cutoff).Delete(&Event{}).Error
}
