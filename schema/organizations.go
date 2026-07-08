package schema

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
)

const defaultOrgSlug = "default"

type Organization struct {
	ID        string    `gorm:"primaryKey"         json:"id"`
	Name      string    `gorm:"not null"           json:"name"`
	Slug      string    `gorm:"uniqueIndex;not null" json:"slug"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (o *Organization) TableName() string {
	return "organizations_v1"
}

func (o *Organization) CreateDefault(ctx context.Context) error {
	o.ID = uuid.NewString()
	o.Name = defaultOrgSlug
	o.Slug = defaultOrgSlug
	return db.FromContext(ctx).Model(&Organization{}).Create(o).Error
}

func (o *Organization) Create(ctx context.Context) error {
	if o.ID == "" {
		o.ID = uuid.NewString()
	}
	// Retry slug generation on unique constraint violation (max 5 attempts).
	for i := 0; i < 5; i++ {
		if o.Slug == "" {
			o.Slug = generateSlug(o.Name)
		}
		err := db.FromContext(ctx).Model(&Organization{}).Create(o).Error
		if err == nil {
			return nil
		}
		if !isUniqueConstraintErr(err) {
			return err
		}
		o.Slug = ""
	}
	return fmt.Errorf("failed to generate unique slug for organization %q after 5 attempts", o.Name)
}

func (o *Organization) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Organization{}).
		Where("id = ? OR slug = ?", o.ID, o.Slug).
		First(o).
		Error
}

func (o *Organization) GetDefault(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Organization{}).
		Where("slug = ?", defaultOrgSlug).
		Find(o).
		Error
}

func (o *Organization) ListAll(ctx context.Context) ([]Organization, error) {
	var orgs []Organization
	err := db.FromContext(ctx).Model(&Organization{}).Find(&orgs).Error
	return orgs, err
}

func (o *Organization) Update(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Organization{}).
		Where("id = ?", o.ID).
		Updates(o).
		Error
}

func (o *Organization) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Organization{}).
		Where("id = ?", o.ID).
		Delete(o).
		Error
}

// isUniqueConstraintErr returns true if err is a unique constraint violation
// from SQLite or PostgreSQL.
func isUniqueConstraintErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// SQLite: "UNIQUE constraint failed"
	// PostgreSQL: "duplicate key value violates unique constraint"
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "duplicate key value violates unique constraint") ||
		strings.Contains(msg, "23505") // pg error code
}
