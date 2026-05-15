package schema

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"gorm.io/datatypes"
)

var (
	ErrUserInviteIdentifiersNotProvided = errors.New("user invite identifiers not provided")
)

type UserInvite struct {
	ID             string                                       `gorm:"primaryKey" json:"id"`
	InviteCode     string                                       `gorm:"unique" json:"invite_code"`
	InviteURL      string                                       `json:"invite_url"`
	Email          string                                       `json:"email"`
	PlatformRoleID string                                       `json:"platform_role_id"`
	UserGroups     datatypes.JSONType[map[UserGroupID]struct{}] `json:"user_group_ids"`
}

func (u *UserInvite) TableName() string {
	return "user_invites_v1"
}

func (u *UserInvite) Create(ctx context.Context) error {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}

	return db.FromContext(ctx).Model(&UserInvite{}).Create(u).Error
}

func (u *UserInvite) GetByEmail(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserInvite{}).
		Where("email = ?", u.Email).
		First(u).
		Error
}

func (u *UserInvite) ListAll(ctx context.Context, options ...dbtypes.Option) ([]UserInvite, error) {
	var userInvites []UserInvite
	query := db.FromContext(ctx).Model(&UserInvite{})

	for _, option := range options {
		query = option(query)
	}

	err := query.Find(&userInvites).Error
	return userInvites, err
}

func (u *UserInvite) DeleteByEmail(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserInvite{}).
		Where("email = ?", u.Email).
		Delete(u).
		Error
}

func (u *UserInvite) DeleteAll(ctx context.Context) error {
	return db.FromContext(ctx).Exec("DELETE FROM user_invites_v1").Error
}
