package schema

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/gorm"
)

var (
	ErrPendingUserIdentifiersNotProvided = errors.New("pending user identifiers not provided")
)

type PendingUser struct {
	ID                         string      `gorm:"primaryKey" json:"id"`
	Scope                      scope.Scope `gorm:"default:0;uniqueIndex:udx_pending_user_scope_username" json:"scope"`
	ScopeID                    string      `gorm:"default:'';uniqueIndex:udx_pending_user_scope_username" json:"scope_id"`
	Username                   string      `gorm:"uniqueIndex:udx_pending_user_scope_username" json:"username"`
	ExternalIdentityProviderID string      `json:"external_identity_provider_id"`
	CreatedAt                  time.Time   `json:"created_at"`
}

const pendingUsersTable = "pending_users_v1"

func (p *PendingUser) TableName() string {
	return pendingUsersTable
}

// whereIdentifiers builds a query scoped to this pending user's ID or
// username, always qualifying scope/scope_id with the table name and
// requiring them whenever a scope is in effect so an ID lookup can't cross
// scope boundaries.
func (p *PendingUser) whereIdentifiers(ctx context.Context, level scope.Scope, scopeID string) (*gorm.DB, error) {
	if p.ID == "" && p.Username == "" {
		return nil, ErrPendingUserIdentifiersNotProvided
	}

	query := db.FromContext(ctx).Model(&PendingUser{})
	if p.ID != "" {
		query = query.Where(fmt.Sprintf("%s.id = ?", pendingUsersTable), p.ID)
		if scopeID != "" {
			query = query.Where(fmt.Sprintf("%s.scope = ? AND %s.scope_id = ?", pendingUsersTable, pendingUsersTable), level, scopeID)
		}
		return query, nil
	}

	if scopeID == "" {
		return nil, ErrPendingUserIdentifiersNotProvided
	}
	return query.Where(fmt.Sprintf("%s.username = ? AND %s.scope = ? AND %s.scope_id = ?", pendingUsersTable, pendingUsersTable, pendingUsersTable), p.Username, level, scopeID), nil
}

func (p *PendingUser) Create(ctx context.Context) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}

	return db.FromContext(ctx).Model(&PendingUser{}).Create(p).Error
}

func (p *PendingUser) Exists(ctx context.Context) (bool, error) {
	query, err := p.whereIdentifiers(ctx, scope.Level(ctx), scope.ID(ctx))
	if err != nil {
		return false, err
	}

	var count int64
	err = query.Count(&count).Error
	return count > 0, err
}

func (p *PendingUser) Get(ctx context.Context) error {
	query, err := p.whereIdentifiers(ctx, scope.Level(ctx), scope.ID(ctx))
	if err != nil {
		return err
	}

	return query.First(p).Error
}

func (p *PendingUser) ListAll(ctx context.Context, options ...dbtypes.Option) ([]PendingUser, error) {
	var pendingUsers []PendingUser
	query := db.FromContext(ctx).Model(&PendingUser{})

	if scopeID := scope.ID(ctx); scopeID != "" {
		options = append(options,
			dbtypes.WithFilter(fmt.Sprintf("%s.scope", pendingUsersTable), scope.Level(ctx)),
			dbtypes.WithFilter(fmt.Sprintf("%s.scope_id", pendingUsersTable), scopeID),
		)
	}

	for _, option := range options {
		query = option(query)
	}

	err := query.Find(&pendingUsers).Error
	return pendingUsers, err
}

func (p *PendingUser) Delete(ctx context.Context) error {
	query, err := p.whereIdentifiers(ctx, scope.Level(ctx), scope.ID(ctx))
	if err != nil {
		return err
	}
	return query.Delete(p).Error
}

func (p *PendingUser) DeleteAll(ctx context.Context) error {
	if scopeID := scope.ID(ctx); scopeID != "" {
		return db.FromContext(ctx).Exec("DELETE FROM pending_users_v1 WHERE scope = ? AND scope_id = ?", scope.Level(ctx), scopeID).Error
	}
	return db.FromContext(ctx).Exec("DELETE FROM pending_users_v1").Error
}
