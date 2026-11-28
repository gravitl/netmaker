package schema

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
)

const notificationChannelsTable = "notification_channels_v1"

type ChannelType string

const (
	ChannelEmail   ChannelType = "email"
	ChannelWebhook ChannelType = "webhook"
	ChannelSlack   ChannelType = "slack"
	ChannelTeams   ChannelType = "teams"
)

type NotificationChannel struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	TenantID  string         `gorm:"default:'';index" json:"tenant_id"`
	Name      string         `gorm:"not null" json:"name"`
	Type      ChannelType    `gorm:"not null" json:"type"`
	Config    datatypes.JSON `gorm:"not null" json:"config"`
	Enabled   bool           `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

func (c *NotificationChannel) TableName() string {
	return notificationChannelsTable
}

func (c *NotificationChannel) Create(ctx context.Context) error {
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	if tenantID := scope.ID(ctx); tenantID != "" && c.TenantID == "" {
		c.TenantID = tenantID
	}
	return db.FromContext(ctx).Create(c).Error
}

func (c *NotificationChannel) Get(ctx context.Context) error {
	query := db.FromContext(ctx).Model(&NotificationChannel{}).Where("id = ?", c.ID)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", notificationChannelsTable), tenantID)(query)
	}
	return query.First(c).Error
}

func (c *NotificationChannel) Update(ctx context.Context) error {
	query := db.FromContext(ctx).Model(&NotificationChannel{}).Where("id = ?", c.ID)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", notificationChannelsTable), tenantID)(query)
	}
	return query.Updates(c).Error
}

func (c *NotificationChannel) Delete(ctx context.Context) error {
	query := db.FromContext(ctx).Where("id = ?", c.ID)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", notificationChannelsTable), tenantID)(query)
	}
	return query.Delete(&NotificationChannel{}).Error
}

func (c *NotificationChannel) ListByTenant(ctx context.Context) ([]NotificationChannel, error) {
	var channels []NotificationChannel
	query := db.FromContext(ctx).Model(&NotificationChannel{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", notificationChannelsTable), tenantID)(query)
	}
	err := query.Order("created_at DESC").Find(&channels).Error
	return channels, err
}

func (c *NotificationChannel) GetByIDs(ctx context.Context, ids []string) ([]NotificationChannel, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var channels []NotificationChannel
	query := db.FromContext(ctx).Model(&NotificationChannel{}).Where("id IN ?", ids)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", notificationChannelsTable), tenantID)(query)
	}
	err := query.Find(&channels).Error
	return channels, err
}

