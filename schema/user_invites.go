package schema

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
)

type UserInvite struct {
	ID             string                                       `gorm:"primaryKey" json:"id"`
	Scope          scope.Scope                                  `gorm:"default:0;uniqueIndex:udx_user_invite_scope_invite_code" json:"scope"`
	ScopeID        string                                       `gorm:"default:'';uniqueIndex:udx_user_invite_scope_invite_code" json:"scope_id"`
	InviteCode     string                                       `gorm:"uniqueIndex:udx_user_invite_scope_invite_code" json:"invite_code"`
	InviteURL      string                                       `json:"invite_url"`
	Email          string                                       `json:"email"`
	PlatformRoleID string                                       `json:"platform_role_id"`
	UserGroups     datatypes.JSONType[map[UserGroupID]struct{}] `json:"user_group_ids"`
}

const userInvitesTable = "user_invites_v1"

func (u *UserInvite) TableName() string {
	return userInvitesTable
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

	if scopeID := scope.ID(ctx); scopeID != "" {
		options = append(options,
			dbtypes.WithFilter(fmt.Sprintf("%s.scope", userInvitesTable), scope.Level(ctx)),
			dbtypes.WithFilter(fmt.Sprintf("%s.scope_id", userInvitesTable), scopeID),
		)
	}

	for _, option := range options {
		query = option(query)
	}

	err := query.Find(&userInvites).Error
	return userInvites, err
}

func (u *UserInvite) DeleteByEmail(ctx context.Context) error {
	query := db.FromContext(ctx).Model(&UserInvite{}).
		Where("email = ?", u.Email)
	if scopeID := scope.ID(ctx); scopeID != "" {
		query = query.
			Where(fmt.Sprintf("%s.scope = ?", userInvitesTable), scope.Level(ctx)).
			Where(fmt.Sprintf("%s.scope_id = ?", userInvitesTable), scopeID)
	}
	return query.Delete(u).Error
}

func (u *UserInvite) DeleteAll(ctx context.Context) error {
	if scopeID := scope.ID(ctx); scopeID != "" {
		return db.FromContext(ctx).Exec("DELETE FROM user_invites_v1 WHERE scope = ? AND scope_id = ?", scope.Level(ctx), scopeID).Error
	}
	return db.FromContext(ctx).Exec("DELETE FROM user_invites_v1").Error
}
