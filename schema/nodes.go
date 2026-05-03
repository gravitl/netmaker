package schema

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"gorm.io/datatypes"
)

const nodesTable = "nodes_v1"

const (
	// NODE_DELETE - delete node action
	NODE_DELETE = "delete"
	// NODE_IS_PENDING - node pending status
	NODE_IS_PENDING = "pending"
	// NODE_NOOP - node no op action
	NODE_NOOP = "noop"
	// NODE_FORCE_UPDATE - indicates a node should pull all changes
	NODE_FORCE_UPDATE = "force"
)

// TODO: check network and host delete cascade issues.
// TODO: Add gateways list API.
// TODO: Add gateway configs list API.

type Node struct {
	ID                                string   `gorm:"primaryKey"`
	HostID                            string   `gorm:"not null;index"`
	Host                              *Host    `gorm:"foreignKey:HostID;constraint:OnDelete:CASCADE"`
	NetworkID                         string   `gorm:"not null;index"`
	Network                           *Network `gorm:"foreignKey:NetworkID;constraint:OnDelete:CASCADE"`
	Address                           string
	Address6                          string
	Connected                         bool
	Action                            string
	Status                            string
	PendingDelete                     bool
	AutoAssignGateway                 bool
	IsGateway                         bool
	IsAutoRelay                       bool
	AllowRelayingAllTraffic           bool
	RelayedClients                    datatypes.JSONMap
	RelayedIGWClients                 datatypes.JSONMap
	RelayingNodeID                    datatypes.NullString
	RelayingAllTraffic                bool
	AutoRelayedPeers                  datatypes.JSONType[map[string]string]
	Tags                              datatypes.JSONMap
	PostureCheckSeverity              Severity
	PostureCheckLastEvaluationCycleID string
	Metadata                          string
	LastCheckIn                       time.Time
	ExpirationDateTime                time.Time
	CreatedAt                         time.Time
	UpdatedAt                         time.Time
}

func (n *Node) TableName() string {
	return nodesTable
}

func (n *Node) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).Create(n).Error
}

func (n *Node) Get(ctx context.Context, options ...dbtypes.Option) error {
	query := db.FromContext(ctx).Model(&Node{})
	for _, opt := range options {
		query = opt(query)
	}
	return query.Where("id = ?", n.ID).First(n).Error
}

func (n *Node) Exists(ctx context.Context) (bool, error) {
	var exists bool
	err := db.FromContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM nodes_v1 WHERE id = ?)",
		n.ID,
	).Scan(&exists).Error
	return exists, err
}

func (n *Node) GetByHostAndNetwork(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("host_id = ? AND network_id = ?", n.HostID, n.NetworkID).
		First(n).
		Error
}

func (n *Node) GetByNetworkAndAddress(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("network_id = ? AND address = ?", n.NetworkID, n.Address).
		First(n).
		Error
}

func (n *Node) GetByNetworkAndAddress6(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("network_id = ? AND address6 = ?", n.NetworkID, n.Address6).
		First(n).
		Error
}

func (n *Node) Update(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).Where("id = ?", n.ID).Updates(n).Error
}

func (n *Node) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Save(n).Error
}

func (n *Node) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).Where("id = ?", n.ID).Delete(n).Error
}

// TODO: Add pagination APIs

func (n *Node) ListAll(ctx context.Context, options ...dbtypes.Option) ([]Node, error) {
	var nodes []Node
	query := db.FromContext(ctx).Model(&Node{})
	for _, opt := range options {
		query = opt(query)
	}
	err := query.Find(&nodes).Error
	return nodes, err
}

func (n *Node) ListByNetwork(ctx context.Context) ([]Node, error) {
	var nodes []Node
	err := db.FromContext(ctx).Model(&Node{}).Where("network_id = ?", n.NetworkID).Find(&nodes).Error
	return nodes, err
}

func (n *Node) ListByHost(ctx context.Context) ([]Node, error) {
	var nodes []Node
	err := db.FromContext(ctx).Model(&Node{}).Where("host_id = ?", n.HostID).Find(&nodes).Error
	return nodes, err
}

func (n *Node) ListByHostAndNetwork(ctx context.Context) ([]Node, error) {
	var nodes []Node
	err := db.FromContext(ctx).Model(&Node{}).
		Where("host_id = ? AND network_id = ?", n.HostID, n.NetworkID).
		Find(&nodes).
		Error
	return nodes, err
}

func (n *Node) Count(ctx context.Context, options ...dbtypes.Option) (int, error) {
	var count int64
	query := db.FromContext(ctx).Model(&Node{})
	for _, opt := range options {
		query = opt(query)
	}
	err := query.Count(&count).Error
	return int(count), err
}

func (n *Node) UpsertViolations(ctx context.Context, violations []PostureCheckViolation) error {
	tx := db.FromContext(ctx).Begin()
	err := tx.Where("node_id = ?", n.ID).Delete(&PostureCheckViolation{}).Error
	if err != nil {
		rollbackErr := tx.Rollback().Error
		if rollbackErr != nil {
			err = fmt.Errorf("%v; rollback failed: %v", err, rollbackErr)
		}

		return err
	}

	if len(violations) > 0 {
		err := tx.Create(&violations).Error
		if err != nil {
			rollbackErr := tx.Rollback().Error
			if rollbackErr != nil {
				err = fmt.Errorf("%v; rollback failed: %v", err, rollbackErr)
			}

			return err
		}
	}

	return tx.Commit().Error
}

func (n *Node) ListViolations(ctx context.Context) ([]PostureCheckViolation, error) {
	var violations []PostureCheckViolation
	err := db.FromContext(ctx).Model(&PostureCheckViolation{}).
		Where("node_id = ?", n.ID).
		Find(&violations).
		Error
	return violations, err
}

func (n *Node) DeleteViolations(ctx context.Context) error {
	return db.FromContext(ctx).Model(&PostureCheckViolation{}).
		Where("node_id = ?", n.ID).
		Delete(&PostureCheckViolation{}).
		Error
}

func (n *Node) UpdateConnectedStatus(ctx context.Context, options ...dbtypes.Option) error {
	query := db.FromContext(ctx).Model(&Node{})
	for _, opt := range options {
		query = opt(query)
	}
	if n.ID != "" {
		query = query.Where("id = ?", n.ID)
	}
	return query.Update("connected", n.Connected).Error
}

func (n *Node) MarkForDeletion(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		Update("pending_delete", true).
		Update("action", NODE_DELETE).
		Error
}

func (n *Node) UpdateRelayingNode(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		Update("relaying_node_id", n.RelayingNodeID).
		Error
}

func (n *Node) UpdateRelayedClients(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		Update("relayed_clients", n.RelayedClients).
		Error
}

func (n *Node) UpdateTags(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		Update("tags", n.Tags).
		Error
}

func (n *Node) UpdateLastCheckIn(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		Update("last_check_in", n.LastCheckIn).
		Error
}
