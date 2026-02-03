package schema

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

var (
	ErrNetworkIdentifiersNotProvided = errors.New("network identifiers not provided")
)

type Network struct {
	ID                  string `gorm:"primaryKey"`
	Name                string `gorm:"unique"`
	AddressRange        string
	AddressRange6       string
	DefaultKeepAlive    time.Duration
	DefaultACL          string
	DefaultMTU          int32
	AutoJoin            string
	AutoRemove          string
	AutoRemoveTags      datatypes.JSONSlice[string]
	AutoRemoveThreshold time.Duration
	NodesUpdatedAt      time.Time
	CreatedBy           string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (n *Network) TableName() string {
	return "networks_v1"
}

func (n *Network) Create(ctx context.Context) error {
	if n.ID == "" {
		n.ID = uuid.NewString()
	}

	return db.FromContext(ctx).Model(&Network{}).Create(n).Error
}

func (n *Network) Get(ctx context.Context) error {
	if n.ID == "" && n.Name == "" {
		return ErrNetworkIdentifiersNotProvided
	}

	return db.FromContext(ctx).Model(&Network{}).
		Where("id = ? OR name = ?", n.ID, n.Name).
		First(n).
		Error
}

func (n *Network) Count(ctx context.Context) (int, error) {
	var count int64
	err := db.FromContext(ctx).Model(&Network{}).Count(&count).Error
	return int(count), err
}

func (n *Network) ListAll(ctx context.Context) ([]Network, error) {
	var networks []Network
	err := db.FromContext(ctx).Model(&Network{}).Find(&networks).Error
	return networks, err
}

func (n *Network) Update(ctx context.Context) error {
	if n.ID == "" && n.Name == "" {
		return ErrNetworkIdentifiersNotProvided
	}

	return db.FromContext(ctx).Model(&Network{}).
		Where("id = ? OR name = ?", n.ID, n.Name).
		Updates(n).Error
}

func (n *Network) Delete(ctx context.Context) error {
	if n.ID == "" && n.Name == "" {
		return ErrNetworkIdentifiersNotProvided
	}

	return db.FromContext(ctx).Model(&Network{}).
		Where("id = ? OR name = ?", n.ID, n.Name).
		Delete(n).Error
}

func (n *Network) UpdateNodesUpdatedAt(ctx context.Context) error {
	if n.ID == "" && n.Name == "" {
		return ErrNetworkIdentifiersNotProvided
	}

	return db.FromContext(ctx).Model(&Network{}).
		Where("id = ? OR name = ?", n.ID, n.Name).
		Updates(map[string]interface{}{
			"nodes_updated_at": n.NodesUpdatedAt,
		}).Error
}
