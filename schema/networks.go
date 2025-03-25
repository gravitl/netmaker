package schema

import (
	"context"
	"github.com/gravitl/netmaker/db"
	"time"
)

type Network struct {
	ID                  string `gorm:"id;primaryKey"`
	IsIPv4              string `gorm:"default:'yes'"`
	IsIPv6              string `gorm:"default:'no'"`
	AddressRange        string
	AddressRange6       string
	NodeLimit           int32  `gorm:"default:999999999"`
	AllowManualSignUp   string `gorm:"default:'no'"`
	DefaultInterface    string
	DefaultPostDown     string
	DefaultUDPHolePunch string   `gorm:"default:'no'"`
	DefaultACL          string   `gorm:"default:'yes'"`
	DefaultListenPort   int32    `gorm:"default:51821"`
	DefaultKeepalive    int32    `gorm:"default:20"`
	DefaultMTU          int32    `gorm:"default:1280"`
	NameServers         []string `gorm:"serializer:json"`
	NodesLastModified   time.Time
	NetworkLastModified time.Time
}

func (n *Network) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Network{}).Create(n).Error
}

func (n *Network) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Network{}).Where("id = ?", n.ID).First(n).Error
}

func (n *Network) ListAll(ctx context.Context) ([]Network, error) {
	var networks []Network
	err := db.FromContext(ctx).Model(&Network{}).Find(&networks).Error
	return networks, err
}

func (n *Network) Count(ctx context.Context) (int, error) {
	var count int64
	err := db.FromContext(ctx).Model(&Network{}).Count(&count).Error
	return int(count), err
}

func (n *Network) Update(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Network{}).Where("id = ?", n.ID).Updates(n).Error
}

func (n *Network) UpdateNodesLastModified(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Network{}).
		Where("id = ?", n.ID).
		Update("nodes_last_modified", n.NodesLastModified).Error
}

func (n *Network) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Network{}).Where("id = ?", n.ID).Delete(n).Error
}
