package schema

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"gorm.io/datatypes"
)

type Activity struct {
	ID        string           `gorm:"primaryKey" json:"id"`
	Action    models.Action    `gorm:"action" json:"action"`
	Source    datatypes.JSON   `gorm:"source" json:"source"`
	Origin    models.Origin    `gorm:"origin" json:"origin"`
	Target    datatypes.JSON   `gorm:"target" json:"target"`
	NetworkID models.NetworkID `gorm:"network_id" json:"network_id"`
	TimeStamp time.Time        `gorm:"time_stamp" json:"time_stamp"`
}

func (a *Activity) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Activity{}).First(&a).Where("id = ?", a.ID).Error
}

func (a *Activity) Update(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Activity{}).Where("id = ?", a.ID).Updates(&a).Error
}

func (a *Activity) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Activity{}).Create(&a).Error
}

func (a *Activity) List(ctx context.Context) (ats []Activity, err error) {
	err = db.FromContext(ctx).Model(&Activity{}).Find(&ats).Error
	return
}
