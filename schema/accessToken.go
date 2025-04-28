package schema

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
)

// UserAccessToken - token used to access netmaker
type UserAccessToken struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`
	UserName  string    `json:"user_name"`
	ExpiresAt time.Time `json:"expires_at"`
	LastUsed  time.Time `json:"last_used"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

func (a *UserAccessToken) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserAccessToken{}).First(&a).Where("id = ?", a.ID).Error
}

func (a *UserAccessToken) Update(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserAccessToken{}).Where("id = ?", a.ID).Updates(&a).Error
}

func (a *UserAccessToken) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserAccessToken{}).Create(&a).Error
}

func (a *UserAccessToken) List(ctx context.Context) (ats []UserAccessToken, err error) {
	err = db.FromContext(ctx).Model(&UserAccessToken{}).Find(&ats).Error
	return
}

func (a *UserAccessToken) ListByUser(ctx context.Context) (ats []UserAccessToken) {
	db.FromContext(ctx).Model(&UserAccessToken{}).Where("user_name = ?", a.UserName).Find(&ats)
	if ats == nil {
		ats = []UserAccessToken{}
	}
	return
}

func (a *UserAccessToken) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserAccessToken{}).Where("id = ?", a.ID).Delete(&a).Error
}

func (a *UserAccessToken) DeleteAllUserTokens(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserAccessToken{}).Where("user_name = ?", a.UserName).Delete(&a).Error
}
