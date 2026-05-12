package schema

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/db/expr"
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

type NodeStatus string

const (
	OnlineSt     NodeStatus = "online"
	OfflineSt    NodeStatus = "offline"
	WarningSt    NodeStatus = "warning"
	ErrorSt      NodeStatus = "error"
	UnKnown      NodeStatus = "unknown"
	Disconnected NodeStatus = "disconnected"
)

type Node struct {
	ID                                string                                `gorm:"primaryKey" json:"id"`
	HostID                            string                                `gorm:"not null;index" json:"host_id"`
	Host                              *Host                                 `gorm:"foreignKey:HostID;constraint:OnDelete:CASCADE" json:"host,omitempty"`
	NetworkID                         string                                `gorm:"not null;index" json:"network_id"`
	Network                           *Network                              `gorm:"foreignKey:NetworkID;constraint:OnDelete:CASCADE" json:"network,omitempty"`
	Address                           string                                `json:"address"`
	Address6                          string                                `json:"address6"`
	Connected                         bool                                  `json:"connected"`
	Action                            string                                `json:"action"`
	Status                            NodeStatus                            `json:"status"`
	PendingDelete                     bool                                  `json:"pending_delete"`
	AutoAssignGateway                 bool                                  `json:"auto_assign_gateway"`
	IsGateway                         bool                                  `json:"is_gateway"`
	IsAutoRelay                       bool                                  `json:"is_auto_relay"`
	IsInternetGateway                 bool                                  `json:"is_internet_gateway"`
	RelayedClients                    datatypes.JSONMap                     `json:"relayed_clients"`
	RelayedIGWClients                 datatypes.JSONMap                     `json:"relayed_igw_clients"`
	RelayedByNodeID                   *string                               `json:"relayed_by_node_id"`
	IsIGWClient                       bool                                  `json:"is_igw_client"`
	AutoRelayedPeers                  datatypes.JSONType[map[string]string] `json:"auto_relayed_peers"`
	Tags                              datatypes.JSONMap                     `json:"tags"`
	PostureCheckSeverity              Severity                              `json:"posture_check_severity"`
	PostureCheckLastEvaluationCycleID string                                `json:"posture_check_last_evaluation_cycle_id"`
	Metadata                          string                                `json:"metadata"`
	LastCheckIn                       time.Time                             `json:"last_check_in"`
	ExpirationDateTime                time.Time                             `json:"expiration_date_time"`
	CreatedAt                         time.Time                             `json:"created_at"`
	UpdatedAt                         time.Time                             `json:"updated_at"`
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

func (n *Node) DeleteAll(ctx context.Context) error {
	return db.FromContext(ctx).Exec("DELETE FROM nodes_v1").Error
}

func (n *Node) ListAll(ctx context.Context, options ...dbtypes.Option) ([]Node, error) {
	var nodes []Node
	query := db.FromContext(ctx).Model(&Node{})
	for _, opt := range options {
		query = opt(query)
	}
	err := query.Find(&nodes).Error
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
	if len(violations) > 0 {
		err := db.FromContext(ctx).Model(&PostureCheckViolation{}).Create(&violations).Error
		if err != nil {
			return err
		}
	}

	return db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		Update("posture_check_last_evaluation_cycle_id", n.PostureCheckLastEvaluationCycleID).
		Update("posture_check_severity", n.PostureCheckSeverity).
		Error
}

func (n *Node) ListViolations(ctx context.Context) ([]PostureCheckViolation, error) {
	var violations []PostureCheckViolation
	err := db.FromContext(ctx).Model(&PostureCheckViolation{}).
		Where("node_id = ? AND evaluation_cycle_id = ?", n.ID, n.PostureCheckLastEvaluationCycleID).
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

	updates := make(map[string]interface{})
	updates["connected"] = n.Connected
	updates["status"] = n.Status
	if n.Connected {
		updates["last_check_in"] = n.LastCheckIn
	}
	return query.Updates(updates).Error
}

func (n *Node) MarkForDeletion(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		Update("pending_delete", true).
		Update("action", NODE_DELETE).
		Error
}

func (n *Node) SetInternetGateway(ctx context.Context) error {
	err := db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		UpdateColumn("is_internet_gateway", n.IsInternetGateway).
		UpdateColumn("relayed_clients", expr.Merge("relayed_clients", n.RelayedClients)).
		UpdateColumn("relayed_igw_clients", expr.Merge("relayed_igw_clients", n.RelayedIGWClients)).
		Error
	if err != nil {
		return err
	}

	relayedIGWClients := make([]string, 0, len(n.RelayedIGWClients))
	for relayedIGWClientID := range n.RelayedIGWClients {
		relayedIGWClients = append(relayedIGWClients, relayedIGWClientID)
	}

	return db.FromContext(ctx).Model(&Node{}).
		Where("id IN ?", relayedIGWClients).
		UpdateColumn("is_igw_client", true).
		UpdateColumn("relayed_by_node_id", n.ID).
		Error
}

func (n *Node) UpdateRelayingNode(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		Update("relayed_by_node_id", n.RelayedByNodeID).
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

func (n *Node) SetRelayedClients(ctx context.Context) error {
	err := db.FromContext(ctx).Model(&Node{}).
		Where("relayed_by_node_id = ?", n.ID).
		Update("relayed_by_node_id", nil).
		Error
	if err != nil {
		return err
	}

	err = db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		Update("relayed_clients", n.RelayedClients).
		Error
	if err != nil {
		return err
	}

	if len(n.RelayedClients) > 0 {
		clientIDs := make([]string, 0, len(n.RelayedClients))
		for clientID := range n.RelayedClients {
			clientIDs = append(clientIDs, clientID)
		}

		err = db.FromContext(ctx).Model(&Node{}).
			Where("id IN ?", clientIDs).
			Update("relayed_by_node_id", n.ID).
			Error
		if err != nil {
			return err
		}
	}

	return nil
}

func (n *Node) AssignInternetGateway(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		Updates(map[string]interface{}{
			"is_igw_client":      n.IsIGWClient,
			"relayed_by_node_id": n.RelayedByNodeID,
		}).Error
}

func (n *Node) ResetAutoAssignGateway(ctx context.Context) error {
	if n.NetworkID == "" {
		return fmt.Errorf("ResetAutoAssignGateway: NetworkID not set")
	}

	err := db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		Update("auto_assign_gateway", false).
		Error
	if err != nil {
		return err
	}

	return db.FromContext(ctx).Model(&Node{}).
		Where("network_id = ?", n.NetworkID).
		Where(datatypes.JSONQuery("relayed_clients").HasKey(n.ID)).
		UpdateColumn("relayed_clients", expr.Remove("relayed_clients", n.ID)).
		Error
}

func (n *Node) ResetAutoRelayedPeers(ctx context.Context) error {
	if n.NetworkID == "" {
		return fmt.Errorf("ResetAutoAssignGateway: NetworkID not set")
	}

	err := db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		Update("auto_relayed_peers", datatypes.JSONMap{}).
		Error
	if err != nil {
		return err
	}

	return db.FromContext(ctx).Model(&Node{}).
		Where("network_id = ?", n.NetworkID).
		Where(datatypes.JSONQuery("auto_relayed_peers").HasKey(n.ID)).
		UpdateColumn("auto_relayed_peers", expr.Remove("auto_relayed_peers", n.ID)).
		Error
}
