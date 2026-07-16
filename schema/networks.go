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
	"gorm.io/datatypes"
	"gorm.io/gorm"
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
	TenantID      string `gorm:"default:'';uniqueIndex:udx_network_tenant_name" json:"tenant_id"`
	Name          string `gorm:"uniqueIndex:udx_network_tenant_name" json:"netid"`
	AddressRange  string `json:"addressrange"`
	AddressRange6 string `json:"addressrange6"`
	// in seconds.
	DefaultKeepAlive int                         `gorm:"default:20" json:"defaultkeepalive"`
	DefaultMTU       int32                       `gorm:"default:1280" json:"defaultmtu"`
	AutoJoin         bool                        `json:"auto_join"`
	AutoRemove       bool                        `json:"auto_remove"`
	AutoRemoveTags   datatypes.JSONSlice[string] `json:"auto_remove_tags"`
	// in minutes
	AutoRemoveThreshold         int                              `json:"auto_remove_threshold"`
	JITEnabled                  bool                             `json:"jit_enabled"`
	JITUserGroupIDs             datatypes.JSONSlice[UserGroupID] `json:"jit_user_group_ids"`
	VirtualNATPoolIPv4          string                           `json:"virtual_nat_pool_ipv4"`
	VirtualNATSitePrefixLenIPv4 int                              `json:"virtual_nat_site_prefixlen_ipv4"`
	NodesUpdatedAt              time.Time                        `json:"nodes_updated_at"`
	CreatedBy                   string                           `json:"created_by"`
	CreatedAt                   time.Time                        `json:"created_at"`
	UpdatedAt                   time.Time                        `json:"updated_at"`
}

const networksTable = "networks_v1"

func (n *Network) TableName() string {
	return networksTable
}

func (n *Network) baseIdentifierQuery(ctx context.Context) (*gorm.DB, error) {
	tenantID := scope.ID(ctx)
	if n.ID == "" && (n.Name == "" || tenantID == "") {
		return nil, ErrNetworkIdentifiersNotProvided
	}

	query := db.FromContext(ctx).Model(&Network{})
	if n.ID != "" {
		query = query.Where(fmt.Sprintf("%s.id = ?", networksTable), n.ID)
		if tenantID != "" {
			query = query.Where(fmt.Sprintf("%s.tenant_id = ?", networksTable), tenantID)
		}
		return query, nil
	}

	return query.Where(fmt.Sprintf("%s.name = ? AND %s.tenant_id = ?", networksTable, networksTable), n.Name, tenantID), nil
}

func (n *Network) Create(ctx context.Context) error {
	if n.ID == "" {
		n.ID = uuid.NewString()
	}

	return db.FromContext(ctx).Model(&Network{}).Create(n).Error
}

func (n *Network) Get(ctx context.Context) error {
	query, err := n.baseIdentifierQuery(ctx)
	if err != nil {
		return err
	}

	var network Network
	err = query.First(&network).Error
	if err != nil {
		return err
	}

	*n = network
	return nil
}

func (n *Network) Count(ctx context.Context) (int, error) {
	var count int64
	query := db.FromContext(ctx).Model(&Network{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", networksTable), tenantID)(query)
	}
	err := query.Count(&count).Error
	return int(count), err
}

func (n *Network) ListAll(ctx context.Context) ([]Network, error) {
	var networks []Network
	query := db.FromContext(ctx).Model(&Network{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", networksTable), tenantID)(query)
	}
	err := query.Find(&networks).Error
	return networks, err
}

func (n *Network) Update(ctx context.Context) error {
	query, err := n.baseIdentifierQuery(ctx)
	if err != nil {
		return err
	}

	return query.
		Updates(map[string]interface{}{
			"default_keep_alive":               n.DefaultKeepAlive,
			"default_mtu":                      n.DefaultMTU,
			"auto_join":                        n.AutoJoin,
			"auto_remove":                      n.AutoRemove,
			"auto_remove_tags":                 n.AutoRemoveTags,
			"auto_remove_threshold":            n.AutoRemoveThreshold,
			"jit_enabled":                      n.JITEnabled,
			"jit_user_group_ids":               n.JITUserGroupIDs,
			"virtual_nat_pool_ipv4":            n.VirtualNATPoolIPv4,
			"virtual_nat_site_prefix_len_ipv4": n.VirtualNATSitePrefixLenIPv4,
			"nodes_updated_at":                 n.NodesUpdatedAt,
		}).
		Error
}

func (n *Network) Delete(ctx context.Context) error {
	query, err := n.baseIdentifierQuery(ctx)
	if err != nil {
		return err
	}

	return query.Delete(n).Error
}

func (n *Network) DeleteAll(ctx context.Context) error {
	if tenantID := scope.ID(ctx); tenantID != "" {
		return db.FromContext(ctx).Exec("DELETE FROM networks_v1 WHERE tenant_id = ?", tenantID).Error
	}
	return db.FromContext(ctx).Exec("DELETE FROM networks_v1").Error
}

func (n *Network) UpdateNodesUpdatedAt(ctx context.Context) error {
	query, err := n.baseIdentifierQuery(ctx)
	if err != nil {
		return err
	}

	return query.
		Updates(map[string]interface{}{
			"nodes_updated_at": n.NodesUpdatedAt,
		}).
		Error
}
