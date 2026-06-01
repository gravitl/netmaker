package schema

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
)

var (
	ErrPendingUserIdentifiersNotProvided = errors.New("pending user identifiers not provided")
)

type PendingUser struct {
	ID                         string    `gorm:"primaryKey" json:"id"`
	Username                   string    `gorm:"unique" json:"username"`
	ExternalIdentityProviderID string    `json:"external_identity_provider_id"`
	CreatedAt                  time.Time `json:"created_at"`
}

func (p *PendingUser) TableName() string {
	return "pending_users_v1"
}

func (p *PendingUser) Create(ctx context.Context) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}

	return db.FromContext(ctx).Model(&PendingUser{}).Create(p).Error
}

func (p *PendingUser) Exists(ctx context.Context) (bool, error) {
	if p.ID == "" && p.Username == "" {
		return false, ErrPendingUserIdentifiersNotProvided
	}

	var exists bool
	err := db.FromContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM pending_users_v1 WHERE id = ? OR username = ?)",
		p.ID,
		p.Username,
	).Scan(&exists).Error
	return exists, err
}

func (p *PendingUser) Get(ctx context.Context) error {
	if p.ID == "" && p.Username == "" {
		return ErrPendingUserIdentifiersNotProvided
	}

	return db.FromContext(ctx).Model(&PendingUser{}).
		Where("id = ? OR username = ?", p.ID, p.Username).
		First(p).
		Error
}

func (p *PendingUser) ListAll(ctx context.Context, options ...dbtypes.Option) ([]PendingUser, error) {
	var pendingUsers []PendingUser
	query := db.FromContext(ctx).Model(&PendingUser{})

	for _, option := range options {
		query = option(query)
	}

	err := query.Find(&pendingUsers).Error
	return pendingUsers, err
}

func (p *PendingUser) Delete(ctx context.Context) error {
	if p.ID == "" && p.Username == "" {
		return ErrPendingUserIdentifiersNotProvided
	}

	return db.FromContext(ctx).Model(&PendingUser{}).
		Where("id = ? OR username = ?", p.ID, p.Username).
		Delete(p).
		Error
}

func (p *PendingUser) DeleteAll(ctx context.Context) error {
	return db.FromContext(ctx).Exec("DELETE FROM pending_users_v1").Error
}
