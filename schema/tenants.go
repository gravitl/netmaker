package schema

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"gorm.io/gorm"
)

type Tenant struct {
	ID             string    `gorm:"primaryKey"           json:"id"`
	Name           string    `gorm:"not null"             json:"name"`
	Slug           string    `gorm:"uniqueIndex;not null" json:"slug"`
	OrganizationID string    `gorm:"not null;index"       json:"organization_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (t *Tenant) TableName() string {
	return "tenants_v1"
}

func (t *Tenant) Create(ctx context.Context) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	// Retry slug generation on unique constraint violation (max 5 attempts).
	for i := 0; i < 5; i++ {
		if t.Slug == "" {
			t.Slug = generateSlug(t.Name)
		}
		err := db.FromContext(ctx).Model(&Tenant{}).Create(t).Error
		if err == nil {
			return nil
		}
		if !isUniqueConstraintErr(err) {
			return err
		}
		t.Slug = ""
	}
	return fmt.Errorf("failed to generate unique slug for tenant %q after 5 attempts", t.Name)
}

func (t *Tenant) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Tenant{}).
		Where("id = ? OR slug = ?", t.ID, t.Slug).
		First(t).Error
}

func (t *Tenant) ListAll(ctx context.Context) ([]Tenant, error) {
	var tenants []Tenant
	err := db.FromContext(ctx).Model(&Tenant{}).Find(&tenants).Error
	return tenants, err
}

func (t *Tenant) ListByOrg(ctx context.Context, orgID string) ([]Tenant, error) {
	var tenants []Tenant
	err := db.FromContext(ctx).Model(&Tenant{}).
		Where("organization_id = ?", orgID).
		Find(&tenants).Error
	return tenants, err
}

func (t *Tenant) Update(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Tenant{}).
		Where("id = ?", t.ID).
		Updates(t).Error
}

func (t *Tenant) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Tenant{}).
		Where("id = ?", t.ID).
		Delete(t).Error
}

// EnsureDefaultTenant creates the default tenant for the given org if none
// exists, returning the tenant (existing or newly created).
func EnsureDefaultTenant(ctx context.Context, orgID string) (*Tenant, error) {
	var tenants []Tenant
	if err := db.FromContext(ctx).Model(&Tenant{}).
		Where("organization_id = ?", orgID).
		Limit(1).Find(&tenants).Error; err != nil {
		return nil, err
	}
	if len(tenants) > 0 {
		return &tenants[0], nil
	}
	tenant := &Tenant{OrganizationID: orgID, Name: "Default", Slug: "default"}
	err := db.FromContext(ctx).Model(&Tenant{}).
		Where(gorm.Model{}).
		FirstOrCreate(tenant, Tenant{Slug: "default", OrganizationID: orgID}).Error
	if err != nil {
		// Slug "default" taken — use generated slug.
		tenant.Slug = ""
		if createErr := tenant.Create(ctx); createErr != nil {
			return nil, createErr
		}
	}
	return tenant, nil
}
