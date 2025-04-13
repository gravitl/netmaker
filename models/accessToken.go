package models

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
)

// accessTokenTableName - access tokens table
const accessTokenTableName = "user_access_tokens"

// AccessToken - token used to access netmaker
type AccessToken struct {
	ID        string    `gorm:"id,primary_key" json:"id"`
	Name      string    `gorm:"name" json:"name"`
	UserName  string    `gorm:"user_name" json:"user_name"`
	ExpiresAt time.Time `gorm:"expires_at" json:"expires_at"`
	LastUsed  time.Time `gorm:"last_used" json:"last_used"`
	CreatedBy string    `gorm:"created_by" json:"created_by"`
	CreatedAt time.Time `gorm:"created_at" json:"created_at"`
}

func (a *AccessToken) Table() string {
	return accessTokenTableName
}

func (a *AccessToken) Get() error {
	return db.FromContext(context.TODO()).Table(a.Table()).First(&a).Where("id = ?", a.ID).Error
}

func (a *AccessToken) Update() error {
	return db.FromContext(context.TODO()).Table(a.Table()).Where("id = ?", a.ID).Updates(&a).Error
}

func (a *AccessToken) Create() error {
	return db.FromContext(context.TODO()).Table(a.Table()).Create(&a).Error
}

func (a *AccessToken) List() (ats []AccessToken, err error) {
	err = db.FromContext(context.TODO()).Table(a.Table()).Find(&ats).Error
	return
}

func (a *AccessToken) ListByUser() (ats []AccessToken) {
	db.FromContext(context.TODO()).Table(a.Table()).Where("user_name = ?", a.UserName).Find(&ats)
	return
}

func (a *AccessToken) Delete() error {
	return db.FromContext(context.TODO()).Table(a.Table()).Where("id = ?", a.ID).Delete(&a).Error
}

func (a *AccessToken) DeleteAllUserTokens() error {
	return db.FromContext(context.TODO()).Table(a.Table()).Where("user_name = ?", a.UserName).Delete(&a).Error

}
