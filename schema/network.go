package schema

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

type Network struct {
	ID                  string `gorm:"primaryKey"`
	IsIPv4              string `gorm:"default:'yes'"`
	IsIPv6              string `gorm:"default:'no'"`
	AddressRange        string
	AddressRange6       string
	NodeLimit           int32  `gorm:"default:999999999"`
	AllowManualSignUp   string `gorm:"default:'no'"`
	DefaultInterface    string
	DefaultPostDown     string
	DefaultUDPHolePunch string `gorm:"default:'no'"`
	DefaultACL          string `gorm:"default:'yes'"`
	DefaultListenPort   int32  `gorm:"default:51821"`
	DefaultKeepalive    int32  `gorm:"default:20"`
	DefaultMTU          int32  `gorm:"default:1280"`
	NameServers         datatypes.JSONSlice[string]
	NodesLastModified   time.Time
	NetworkLastModified time.Time
}

func (n *Network) TableName() string {
	return "networks_v1"
}

func (n *Network) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Network{}).Create(n).Error
}

func (n *Network) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(n).
		Where("id = ?", n.ID).
		First(n).
		Error
}

func (n *Network) GetNodes(ctx context.Context) ([]Node, error) {
	var nodes []Node
	err := db.FromContext(ctx).Model(&Node{}).
		Where("network_id = ?", n.ID).
		Find(&nodes).
		Error

	for i := range nodes {
		// suppress error
		_ = nodes[i].fetchRelations(ctx)
	}

	return nodes, err
}

func (n *Network) ListAll(ctx context.Context) ([]Network, error) {
	var networks []Network
	err := db.FromContext(ctx).Model(&Network{}).Find(&networks).Error
	return networks, err
}

func (n *Network) Exists(ctx context.Context) (bool, error) {
	var exists bool
	err := db.FromContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM networks_v1 WHERE id = ?)",
		n.ID,
	).Scan(&exists).Error
	return exists, err
}

func (n *Network) Count(ctx context.Context) (int, error) {
	var count int64
	err := db.FromContext(ctx).Model(&Network{}).Count(&count).Error
	return int(count), err
}

func (n *Network) CountNodes(ctx context.Context) (int, error) {
	var count int64
	err := db.FromContext(ctx).Model(&Node{}).
		Where("network_id = ?", n.ID).
		Count(&count).
		Error
	return int(count), err
}

func (n *Network) Update(ctx context.Context) error {
	return db.FromContext(ctx).Model(n).Save(n).Error
}

func (n *Network) UpdateNodesLastModified(ctx context.Context) error {
	return db.FromContext(ctx).Model(n).
		Update("nodes_last_modified", n.NodesLastModified).
		Error
}

func (n *Network) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(n).Delete(n).Error
}
