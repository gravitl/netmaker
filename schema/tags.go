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
	"gorm.io/gorm/clause"
)

// TagID is the id tags are referred to by: <network name>.<tag name>.
type TagID string

func (id TagID) String() string { return string(id) }

const (
	OldRemoteAccessTagName = "remote-access-gws"
	GwTagName              = "gateways"
)

const tagsTable = "tags_v1"

var ErrTagIdentifiersNotProvided = errors.New("tag identifiers not provided")

type Tag struct {
	ID       string `gorm:"primaryKey" json:"id"`
	TenantID string `gorm:"default:'';uniqueIndex:udx_tag_tenant_network_name" json:"tenant_id"`
	// NetworkID has no foreign key constraint, tags are deleted explicitly
	// on network deletion.
	NetworkID string    `gorm:"not null;index;uniqueIndex:udx_tag_tenant_network_name" json:"network_id"`
	Network   *Network  `gorm:"foreignKey:NetworkID;constraint:-" json:"network,omitempty"`
	Name      string    `gorm:"not null;uniqueIndex:udx_tag_tenant_network_name" json:"name"`
	ColorCode string    `json:"color_code"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (t *Tag) TableName() string {
	return tagsTable
}

// baseIdentifierQuery scopes the query to the tag identified by its ID, or
// by its name in the network identified by NetworkID.
func (t *Tag) baseIdentifierQuery(ctx context.Context) (*gorm.DB, error) {
	query := db.FromContext(ctx).Model(&Tag{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = query.Where(fmt.Sprintf("%s.tenant_id = ?", tagsTable), tenantID)
	}

	if t.ID != "" {
		return query.Where(fmt.Sprintf("%s.id = ?", tagsTable), t.ID), nil
	}

	if t.Name == "" || t.NetworkID == "" {
		return nil, ErrTagIdentifiersNotProvided
	}

	return query.Where(
		fmt.Sprintf("%s.network_id = ? AND %s.name = ?", tagsTable, tagsTable),
		t.NetworkID, t.Name,
	), nil
}

// networkQuery scopes the query to the tags of the network identified by
// NetworkID, or by Network.Name within the tenant in the context. The latter
// joins the network, and populates the Network of the results.
func (t *Tag) networkQuery(ctx context.Context) (*gorm.DB, error) {
	tenantID := scope.ID(ctx)
	query := db.FromContext(ctx).Model(&Tag{})
	if tenantID != "" {
		query = query.Where(fmt.Sprintf("%s.tenant_id = ?", tagsTable), tenantID)
	}

	if t.NetworkID != "" {
		return query.Where(fmt.Sprintf("%s.network_id = ?", tagsTable), t.NetworkID), nil
	}

	// network names are only unique within a tenant.
	if t.Network == nil || t.Network.Name == "" || tenantID == "" {
		return nil, ErrTagIdentifiersNotProvided
	}

	condition := db.FromContext(ctx).Session(&gorm.Session{NewDB: true}).Where(&Network{Name: t.Network.Name})
	return query.InnerJoins("Network", condition), nil
}

func (t *Tag) Create(ctx context.Context) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}

	return db.FromContext(ctx).Model(&Tag{}).Omit(clause.Associations).Create(t).Error
}

// Get fetches the tag by its ID, by its name in the network identified by
// NetworkID, or by its name in the network identified by Network.Name.
func (t *Tag) Get(ctx context.Context) error {
	var query *gorm.DB
	var err error
	if t.ID == "" && t.NetworkID == "" {
		if t.Name == "" {
			return ErrTagIdentifiersNotProvided
		}

		query, err = t.networkQuery(ctx)
		if err == nil {
			query = query.Where(fmt.Sprintf("%s.name = ?", tagsTable), t.Name)
		}
	} else {
		query, err = t.baseIdentifierQuery(ctx)
	}
	if err != nil {
		return err
	}

	var tag Tag
	err = query.First(&tag).Error
	if err != nil {
		return err
	}

	*t = tag
	return nil
}

// Update overwrites all the fields of the tag identified by its ID, except
// the tenant and the creation time. It returns gorm.ErrRecordNotFound if the
// tag does not exist.
func (t *Tag) Update(ctx context.Context) error {
	if t.ID == "" {
		return ErrTagIdentifiersNotProvided
	}

	result := db.FromContext(ctx).Model(&Tag{}).
		Where(fmt.Sprintf("%s.id = ?", tagsTable), t.ID).
		Select("*").
		Omit("id", "tenant_id", "created_at", clause.Associations).
		Updates(t)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Delete deletes the tag. It returns gorm.ErrRecordNotFound if the tag does
// not exist.
func (t *Tag) Delete(ctx context.Context) error {
	query, err := t.baseIdentifierQuery(ctx)
	if err != nil {
		return err
	}

	result := query.Delete(&Tag{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ListByNetwork lists the tags of the network identified by NetworkID, or by
// Network.Name.
func (t *Tag) ListByNetwork(ctx context.Context, options ...dbtypes.Option) ([]Tag, error) {
	query, err := t.networkQuery(ctx)
	if err != nil {
		return nil, err
	}

	for _, opt := range options {
		query = opt(query)
	}

	var tags []Tag
	err = query.Find(&tags).Error
	return tags, err
}

// DeleteByNetwork deletes the tags of the network identified by NetworkID.
func (t *Tag) DeleteByNetwork(ctx context.Context) error {
	if t.NetworkID == "" {
		return ErrTagIdentifiersNotProvided
	}

	query := db.FromContext(ctx).Where(fmt.Sprintf("%s.network_id = ?", tagsTable), t.NetworkID)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = query.Where(fmt.Sprintf("%s.tenant_id = ?", tagsTable), tenantID)
	}

	return query.Delete(&Tag{}).Error
}

func (t *Tag) ListAll(ctx context.Context, options ...dbtypes.Option) ([]Tag, error) {
	query := db.FromContext(ctx).Model(&Tag{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		options = append(options, dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", tagsTable), tenantID))
	}

	for _, opt := range options {
		query = opt(query)
	}

	var tags []Tag
	err := query.Find(&tags).Error
	return tags, err
}

func (t *Tag) Count(ctx context.Context, options ...dbtypes.Option) (int, error) {
	query := db.FromContext(ctx).Model(&Tag{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		options = append(options, dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", tagsTable), tenantID))
	}

	for _, opt := range options {
		query = opt(query)
	}

	var count int64
	err := query.Count(&count).Error
	return int(count), err
}

func (t *Tag) DeleteAll(ctx context.Context) error {
	if tenantID := scope.ID(ctx); tenantID != "" {
		return db.FromContext(ctx).Exec(fmt.Sprintf("DELETE FROM %s WHERE tenant_id = ?", tagsTable), tenantID).Error
	}

	return db.FromContext(ctx).Exec(fmt.Sprintf("DELETE FROM %s", tagsTable)).Error
}
