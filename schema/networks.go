package schema

import (
	"context"
	"github.com/gravitl/netmaker/db"
)

type Network struct {
	ID                  string   `gorm:"id;primaryKey"`
	IsIPv4              string   `gorm:"is_ipv4;default:'yes'"`
	IsIPv6              string   `gorm:"is_ipv6;default:'no'"`
	AddressRange        string   `gorm:"address_range"`
	AddressRange6       string   `gorm:"address_range6"`
	NodeLimit           int32    `gorm:"node_limit;default:999999999"`
	AllowManualSignUp   string   `gorm:"allow_manual_sign_up;default:'no'"`
	DefaultInterface    string   `gorm:"default_interface"`
	DefaultPostDown     string   `gorm:"default_post_down"`
	DefaultUDPHolePunch string   `gorm:"default_udp_hole_punch;default:'no'"`
	DefaultACL          string   `gorm:"default_acl;default:'yes'"`
	DefaultListenPort   int32    `gorm:"default_listen_port;default:51821"`
	DefaultKeepalive    int32    `gorm:"default_keepalive;default:20"`
	DefaultMTU          int32    `gorm:"default_mtu;default:1280"`
	NameServers         []string `gorm:"name_servers;serializer:json"`
	NodesLastModified   int64    `gorm:"nodes_last_modified"`
	NetworkLastModified int64    `gorm:"network_last_modified"`
}

func (n *Network) TableName() string {
	return "networks"
}

func (n *Network) Create(ctx context.Context) error {
	return db.FromContext(ctx).Table(n.TableName()).Create(n).Error
}

func (n *Network) Get(ctx context.Context) error {
	return db.FromContext(ctx).Table(n.TableName()).Where("id = ?", n.ID).First(n).Error
}

func (n *Network) ListAll(ctx context.Context) ([]Network, error) {
	var networks []Network
	err := db.FromContext(ctx).Table(n.TableName()).Find(&networks).Error
	return networks, err
}

func (n *Network) Count(ctx context.Context) (int, error) {
	var count int64
	err := db.FromContext(ctx).Table(n.TableName()).Count(&count).Error
	return int(count), err
}

func (n *Network) Update(ctx context.Context) error {
	return db.FromContext(ctx).Table(n.TableName()).Where("id = ?", n.ID).Updates(n).Error
}

func (n *Network) UpdateNodesLastModified(ctx context.Context) error {
	return db.FromContext(ctx).Table(n.TableName()).
		Where("id = ?", n.ID).
		Update("nodes_last_modified", n.NodesLastModified).Error
}

func (n *Network) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Table(n.TableName()).Where("id = ?", n.ID).Delete(n).Error
}
