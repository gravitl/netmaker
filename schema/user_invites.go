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

type UserInviteType string

const (
	MembershipInvite UserInviteType = "membership"
	ValidationInvite UserInviteType = "validation"
)

type UserInvite struct {
	ID             string                                       `gorm:"primaryKey" json:"id"`
	Scope          scope.Scope                                  `json:"scope"`
	ScopeID        string                                       `gorm:"default:''" json:"scope_id"`
	InviteCode     string                                       `gorm:"uniqueIndex" json:"invite_code"`
	InviteURL      string                                       `json:"invite_url"`
	Email          string                                       `json:"email"`
	PlatformRoleID string                                       `json:"platform_role_id"`
	UserGroups     datatypes.JSONType[map[UserGroupID]struct{}] `json:"user_group_ids"`
	Type           UserInviteType                               `gorm:"default:'membership'" json:"type"`
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

func (u *UserInvite) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserInvite{}).
		Where("invite_code = ? AND type = ?", u.InviteCode, MembershipInvite).
		Find(u).Error
}

func (u *UserInvite) GetValidationInvite(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserInvite{}).
		Where("invite_code = ? AND email = ? AND type = ?", u.InviteCode, u.Email, ValidationInvite).
		First(u).Error
}

func (u *UserInvite) GetPendingValidationInvite(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserInvite{}).
		Where("email = ? AND type = ?", u.Email, ValidationInvite).
		First(u).Error
}

func (u *UserInvite) GetByEmail(ctx context.Context) error {
	query := db.FromContext(ctx).Model(&UserInvite{}).
		Where("email = ? AND type = ?", u.Email, MembershipInvite)
	if scopeID := scope.ID(ctx); scopeID != "" {
		query = query.
			Where(fmt.Sprintf("%s.scope = ?", userInvitesTable), scope.Level(ctx)).
			Where(fmt.Sprintf("%s.scope_id = ?", userInvitesTable), scopeID)
	}
	return query.First(u).Error
}

func (u *UserInvite) ListAll(ctx context.Context, options ...dbtypes.Option) ([]UserInvite, error) {
	var userInvites []UserInvite
	query := db.FromContext(ctx).Model(&UserInvite{}).Where("type = ?", MembershipInvite)

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
		Where("email = ? AND type = ?", u.Email, MembershipInvite)
	if scopeID := scope.ID(ctx); scopeID != "" {
		query = query.
			Where(fmt.Sprintf("%s.scope = ?", userInvitesTable), scope.Level(ctx)).
			Where(fmt.Sprintf("%s.scope_id = ?", userInvitesTable), scopeID)
	}
	return query.Delete(u).Error
}

func (u *UserInvite) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserInvite{}).
		Where("id = ?", u.ID).
		Delete(u).Error
}

func (u *UserInvite) DeleteAll(ctx context.Context) error {
	if scopeID := scope.ID(ctx); scopeID != "" {
		return db.FromContext(ctx).Exec("DELETE FROM user_invites_v1 WHERE scope = ? AND scope_id = ? AND type = ?", scope.Level(ctx), scopeID, MembershipInvite).Error
	}
	return db.FromContext(ctx).Exec("DELETE FROM user_invites_v1 WHERE type = ?", MembershipInvite).Error
}
