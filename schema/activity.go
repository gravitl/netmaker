package schema

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"gorm.io/datatypes"
)

type Action string

const (
	Create Action = "CREATE"
	Update Action = "UPDATE"
	Delete Action = "DELETE"
)

type SubjectType string

const (
	UserSub    SubjectType = "USER"
	DeviceSub  SubjectType = "DEVICE"
	NodeSub    SubjectType = "NODE"
	SettingSub SubjectType = "SETTING"
	AclSub     SubjectType = "ACLs"
	EgressSub  SubjectType = "EGRESS"
	NetworkSub SubjectType = "NETWORK"
)

type Origin string

const (
	Dashboard Origin = "DASHBOARD"
	Api       Origin = "API"
	NMCTL     Origin = "NMCTL"
	ClientApp Origin = "CLIENT-APP"
)

type Subject struct {
	ID   string      `json:"id"`
	Name string      `json:"name"`
	Type SubjectType `json:"subject_type"`
}

type Activity struct {
	ID        string           `gorm:"primaryKey" json:"id"`
	Action    Action           `gorm:"action" json:"action"`
	Source    datatypes.JSON   `gorm:"source" json:"source"`
	Origin    string           `gorm:"origin" json:"origin"`
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
