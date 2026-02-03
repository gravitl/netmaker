package schema

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
)

type Scope string

const (
	Scope_Global       = "global"
	Scope_Organization = "organization"
	Scope_Tenant       = "tenant"
	Scope_Group        = "group"
	Scope_Network      = "network"
)

type PrincipalType string

const (
	Principal_User  = "user"
	Principal_Group = "group"
)

type RoleID string

type GroupRoleID string

const (
	GroupRole_Member = "member"
)

type AccessGrant struct {
	ID            string `gorm:"primaryKey"`
	PrincipalType PrincipalType
	PrincipalID   string
	Scope         Scope
	ScopeID       string
	RoleID        RoleID
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (a *AccessGrant) TableName() string {
	return "access_grants_v1"
}

func (a *AccessGrant) Create(ctx context.Context) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}

	return db.FromContext(ctx).Model(&AccessGrant{}).Create(a).Error
}

func (a *AccessGrant) ListByPrincipal(ctx context.Context) ([]AccessGrant, error) {
	var grants []AccessGrant
	err := db.FromContext(ctx).Model(&AccessGrant{}).
		Where("principal_type = ? AND principal_id = ?", a.PrincipalType, a.PrincipalID).
		Find(&grants).Error
	return grants, err
}
