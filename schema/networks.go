package schema

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

type NetworkID string

func (n NetworkID) String() string {
	return string(n)
}

const AllNetworks NetworkID = "all_networks"

var (
	ErrNetworkIdentifiersNotProvided = errors.New("network identifiers not provided")
)

// Network schema.
//
// NOTE: json tags are different from field names to ensure compatibility with the older model.
type Network struct {
	ID            string `gorm:"primaryKey" json:"id"`
	Name          string `gorm:"unique" json:"netid"`
	AddressRange  string `json:"addressrange"`
	AddressRange6 string `json:"addressrange6"`
	// in seconds.
	DefaultKeepAlive int                         `gorm:"default:20" json:"defaultkeepalive"`
	DefaultACL       string                      `gorm:"default:yes" json:"defaultacl"`
	DefaultMTU       int32                       `gorm:"default:1280" json:"defaultmtu"`
	AutoJoin         bool                        `json:"auto_join"`
	AutoRemove       bool                        `json:"auto_remove"`
	AutoRemoveTags   datatypes.JSONSlice[string] `json:"auto_remove_tags"`
	// in minutes
	AutoRemoveThreshold         int       `json:"auto_remove_threshold"`
	JITEnabled                  bool      `json:"jit_enabled"`
	VirtualNATPoolIPv4          string    `json:"virtual_nat_pool_ipv4"`
	VirtualNATSitePrefixLenIPv4 int       `json:"virtual_nat_site_prefixlen_ipv4"`
	NodesUpdatedAt              time.Time `json:"nodes_updated_at"`
	CreatedBy                   string    `json:"created_by"`
	CreatedAt                   time.Time `json:"created_at"`
	UpdatedAt                   time.Time `json:"updated_at"`
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
		Updates(map[string]interface{}{
			"default_keep_alive":               n.DefaultKeepAlive,
			"default_acl":                      n.DefaultACL,
			"default_mtu":                      n.DefaultMTU,
			"auto_join":                        n.AutoJoin,
			"auto_remove":                      n.AutoRemove,
			"auto_remove_tags":                 n.AutoRemoveTags,
			"auto_remove_threshold":            n.AutoRemoveThreshold,
			"jit_enabled":                      n.JITEnabled,
			"virtual_nat_pool_ipv4":            n.VirtualNATPoolIPv4,
			"virtual_nat_site_prefix_len_ipv4": n.VirtualNATSitePrefixLenIPv4,
			"nodes_updated_at":                 n.NodesUpdatedAt,
		}).
		Error
}

func (n *Network) Delete(ctx context.Context) error {
	if n.ID == "" && n.Name == "" {
		return ErrNetworkIdentifiersNotProvided
	}

	return db.FromContext(ctx).Model(&Network{}).
		Where("id = ? OR name = ?", n.ID, n.Name).
		Delete(n).
		Error
}

func (n *Network) UpdateNodesUpdatedAt(ctx context.Context) error {
	if n.ID == "" && n.Name == "" {
		return ErrNetworkIdentifiersNotProvided
	}

	return db.FromContext(ctx).Model(&Network{}).
		Where("id = ? OR name = ?", n.ID, n.Name).
		Updates(map[string]interface{}{
			"nodes_updated_at": n.NodesUpdatedAt,
		}).
		Error
}
