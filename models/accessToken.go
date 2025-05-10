package models

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
)

// accessTokenTableName - access tokens table
const accessTokenTableName = "user_access_tokens"

// UserAccessToken - token used to access netmaker
type UserAccessToken struct {
	ID        string    `gorm:"id,primary_key" json:"id"`
	Name      string    `gorm:"name" json:"name"`
	UserName  string    `gorm:"user_name" json:"user_name"`
	ExpiresAt time.Time `gorm:"expires_at" json:"expires_at"`
	LastUsed  time.Time `gorm:"last_used" json:"last_used"`
	CreatedBy string    `gorm:"created_by" json:"created_by"`
	CreatedAt time.Time `gorm:"created_at" json:"created_at"`
}

func (a *UserAccessToken) Table() string {
	return accessTokenTableName
}

func (a *UserAccessToken) Get() error {
	return db.FromContext(context.TODO()).Table(a.Table()).First(&a).Where("id = ?", a.ID).Error
}

func (a *UserAccessToken) Update() error {
	return db.FromContext(context.TODO()).Table(a.Table()).Where("id = ?", a.ID).Updates(&a).Error
}

func (a *UserAccessToken) Create() error {
	return db.FromContext(context.TODO()).Table(a.Table()).Create(&a).Error
}

func (a *UserAccessToken) List() (ats []UserAccessToken, err error) {
	err = db.FromContext(context.TODO()).Table(a.Table()).Find(&ats).Error
	return
}

func (a *UserAccessToken) ListByUser() (ats []UserAccessToken) {
	db.FromContext(context.TODO()).Table(a.Table()).Where("user_name = ?", a.UserName).Find(&ats)
	if ats == nil {
		ats = []UserAccessToken{}
	}
	return
}

func (a *UserAccessToken) Delete() error {
	return db.FromContext(context.TODO()).Table(a.Table()).Where("id = ?", a.ID).Delete(&a).Error
}

func (a *UserAccessToken) DeleteAllUserTokens() error {
	return db.FromContext(context.TODO()).Table(a.Table()).Where("user_name = ? OR created_by = ?", a.UserName, a.UserName).Delete(&a).Error

}
