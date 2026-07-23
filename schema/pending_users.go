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
// username, always qualifying tenant_id with the table name and requiring it
// whenever a tenant is in scope so an ID lookup can't cross tenant boundaries.
func (p *PendingUser) whereIdentifiers(ctx context.Context, tenantID string) (*gorm.DB, error) {
	if p.ID == "" && p.Username == "" {
		return nil, ErrPendingUserIdentifiersNotProvided
	}

	query := db.FromContext(ctx).Model(&PendingUser{})
	if p.ID != "" {
		query = query.Where(fmt.Sprintf("%s.id = ?", pendingUsersTable), p.ID)
		if tenantID != "" {
			query = query.Where(fmt.Sprintf("%s.tenant_id = ?", pendingUsersTable), tenantID)
		}
		return query, nil
	}

	if tenantID == "" {
		return nil, ErrPendingUserIdentifiersNotProvided
	}
	return query.Where(fmt.Sprintf("%s.username = ? AND %s.tenant_id = ?", pendingUsersTable, pendingUsersTable), p.Username, tenantID), nil
}

func (p *PendingUser) Create(ctx context.Context) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}

	return db.FromContext(ctx).Model(&PendingUser{}).Create(p).Error
}

func (p *PendingUser) Exists(ctx context.Context) (bool, error) {
	query, err := p.whereIdentifiers(ctx, scope.ID(ctx))
	if err != nil {
		return false, err
	}

	var count int64
	err = query.Count(&count).Error
	return count > 0, err
}

func (p *PendingUser) Get(ctx context.Context) error {
	query, err := p.whereIdentifiers(ctx, scope.ID(ctx))
	if err != nil {
		return err
	}

	return query.First(p).Error
}

func (p *PendingUser) ListAll(ctx context.Context, options ...dbtypes.Option) ([]PendingUser, error) {
	var pendingUsers []PendingUser
	query := db.FromContext(ctx).Model(&PendingUser{})

	if tenantID := scope.ID(ctx); tenantID != "" {
		options = append(options, dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", pendingUsersTable), tenantID))
	}

	for _, option := range options {
		query = option(query)
	}

	err := query.Find(&pendingUsers).Error
	return pendingUsers, err
}

func (p *PendingUser) Delete(ctx context.Context) error {
	query, err := p.whereIdentifiers(ctx, scope.ID(ctx))
	if err != nil {
		return err
	}
	return query.Delete(p).Error
}

func (p *PendingUser) DeleteAll(ctx context.Context) error {
	if tenantID := scope.ID(ctx); tenantID != "" {
		return db.FromContext(ctx).Exec("DELETE FROM pending_users_v1 WHERE tenant_id = ?", tenantID).Error
	}
	return db.FromContext(ctx).Exec("DELETE FROM pending_users_v1").Error
}
