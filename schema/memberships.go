package schema

import (
	"context"

	"github.com/gravitl/netmaker/db"
)

type Memberships struct {
	ID     string `gorm:"primaryKey"`
	UserID string
	// ScopeID is the identifier for the specific scope of membership we are looking at.
	// Currently, memberships are at the Group scope but later this same table would
	// maintain memberships for orgs and tenants as well.
	ScopeID string
}

func (m *Memberships) TableName() string {
	return "memberships_v1"
}

func (m *Memberships) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Memberships{}).Create(m).Error
}

func (m *Memberships) Exists(ctx context.Context) (bool, error) {
	var exists bool
	err := db.FromContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM memberships_v1 WHERE scope_id = ? AND user_id = ?)",
		m.ScopeID,
		m.UserID,
	).Scan(&exists).Error
	return exists, err
}

func (m *Memberships) ListAllMembers(ctx context.Context) ([]Memberships, error) {
	var members []Memberships
	err := db.FromContext(ctx).Model(&Memberships{}).
		Where("scope_id = ?", m.ScopeID).
		Find(&members).
		Error
	return members, err
}

func (m *Memberships) ListAllMemberships(ctx context.Context) ([]Memberships, error) {
	var members []Memberships
	err := db.FromContext(ctx).Model(&Memberships{}).
		Where("user_id = ?", m.UserID).
		Find(&members).
		Error
	return members, err
}

func (m *Memberships) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Memberships{}).Delete(m).Error
}
