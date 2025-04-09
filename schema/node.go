package schema

import (
	"context"
	"database/sql"
	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"time"
)

type Node struct {
	ID            string `gorm:"primaryKey"`
	OwnerID       string
	HostID        string
	LocalAddress  string
	NetworkID     string
	NetworkRange  string
	NetworkRange6 string
	Address       string
	Address6      string
	Server        string
	Connected     bool
	DNSOn         bool
	Action        string

	// GatewayNodeID is the ID of the node that this node uses as a
	// Gateway. If nil, this node does not use any node as its
	// Gateway.
	GatewayNodeID *string
	// GatewayNode is the node that this node uses as a Gateway.
	// If nil, this node does not use any node as its Gateway.
	GatewayNode *Node
	// GatewayFor is the list of Nodes that use this node as a
	// Gateway.
	GatewayFor []Node `gorm:"foreignKey:GatewayNodeID"`
	// GatewayNodeConfig is the Gateway configuration of this
	// node. If nil, this node is not a Gateway node.
	GatewayNodeConfig *datatypes.JSONType[GatewayNodeConfig] `gorm:"foreignKey:GatewayNodeConfigID"`

	// EgressGatewayNodeConfig is the Egress Gateway configuration
	// of this node. If nil, this node is not an Egress Gateway
	// node.
	EgressGatewayNodeConfig *datatypes.JSONType[EgressGatewayNodeConfig] `gorm:"foreignKey:EgressGatewayNodeConfigID"`

	// FailOverNodeID is the ID of the node that this node uses as
	// a FailOver.
	FailOverNodeID *string
	// FailOverNode is the node that this node uses as a FailOver.
	FailOverNode *Node `gorm:"foreignKey:FailOverNodeID"`
	// FailOveredNodes is the list of nodes that use this node as
	// a FailOver.
	FailOveredNodes []Node `gorm:"foreignKey:FailOverNodeID"`
	// IsFailOver indicates if this node is a FailOver node.
	IsFailOver bool

	// InternetGatewayNodeID is the ID of the node that this node
	// uses as an Internet Gateway. If nil, this node does not use
	// any node as its Internet Gateway.
	InternetGatewayNodeID *string
	// InternetGatewayNode is the node that this node uses as an
	// Internet Gateway. If nil, this node does not use any node
	// as its Internet Gateway.
	InternetGatewayNode *Node `gorm:"foreignKey:InternetGatewayNodeID"`
	// InternetGatewayFor is the list of nodes that use this node
	// as a Gateway.
	InternetGatewayFor []Node `gorm:"foreignKey:InternetGatewayNodeID"`
	// IsInternetGateway indicates if this node is an Internet
	// Gateway node.
	IsInternetGateway bool

	Status             string
	DefaultACL         string
	Metadata           string
	Tags               datatypes.JSONSlice[string]
	PendingDelete      bool
	LastModified       time.Time
	LastCheckIn        time.Time
	LastPeerUpdate     time.Time
	ExpirationDateTime time.Time
}

type GatewayNodeConfig struct {
	Range               string
	Range6              string
	PersistentKeepalive int32
	MTU                 int32
	DNS                 string
}

type EgressGatewayNodeConfig struct {
	NatEnabled bool
	Ranges     []RangeWithMetric
}

type RangeWithMetric struct {
	Range  string `json:"range"`
	Metric uint32 `json:"metric"`
}

func (n *Node) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).Create(n).Error
}

func (n *Node) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("nodes.id = ?", n.ID).
		First(n).
		Error
}

func (n *Node) ListAll(ctx context.Context) ([]Node, error) {
	var nodes []Node
	err := db.FromContext(ctx).Model(&Node{}).
		Find(&nodes).
		Error
	return nodes, err
}

func (n *Node) Exists(ctx context.Context) (bool, error) {
	var exists bool
	err := db.FromContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM node WHERE id = ?)",
		n.ID,
	).Scan(&exists).Error
	return exists, err
}

func (n *Node) ExistsWithNetworkAndIPv4(ctx context.Context) (bool, error) {
	var exists bool
	err := db.FromContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM node WHERE network_id = ? AND address = ?)",
		n.NetworkID,
		n.Address,
	).Scan(&exists).Error
	return exists, err
}

func (n *Node) ExistsWithNetworkAndIPv6(ctx context.Context) (bool, error) {
	var exists bool
	err := db.FromContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM node WHERE network_id = ? AND address6 = ?)",
		n.NetworkID,
		n.Address6,
	).Scan(&exists).Error
	return exists, err
}

func (n *Node) Count(ctx context.Context) (int, error) {
	var count int64
	err := db.FromContext(ctx).Model(&Node{}).Count(&count).Error
	return int(count), err
}

func (n *Node) CountByOS(ctx context.Context) (map[string]int, error) {
	rows, err := db.FromContext(ctx).Raw(`
		SELECT hosts.os, COUNT(nodes.id) as count
		FROM hosts LEFT JOIN nodes ON hosts.id = nodes.host_id
		GROUP BY hosts.os
	`).Rows()
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var os string
	var count int
	var countMap = make(map[string]int)
	for rows.Next() {
		err = rows.Scan(&os, &count)
		if err != nil {
			return nil, err
		}

		countMap[os] = count
	}

	return countMap, nil
}

func (n *Node) Update(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Updates(n).
		Error
}

func (n *Node) UpdateLastCheckIn(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		Update("last_check_in", n.LastCheckIn).Error
}

func (n *Node) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Model(&Node{}).Save(n).Error
	})
}

func (n *Node) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).Where("id = ?", n.ID).Delete(n).Error
}

func (n *Node) ConfigureAsGateway(ctx context.Context) error {
	// TODO: Implement Method
	return nil
}

func (n *Node) RemoveGatewayConfig(ctx context.Context) error {
	// TODO: Implement Method
	return nil
}

func (n *Node) ConfigureAsEgressGateway(ctx context.Context) error {
	// TODO: Implement Method
	return nil
}

func (n *Node) RemoveEgressGatewayConfig(ctx context.Context) error {
	// TODO: Implement Method
	return nil
}

func (n *Node) ConfigureAsFailOver(ctx context.Context) error {
	// TODO: Implement Method
	return nil
}

func (n *Node) RemoveFailOverConfig(ctx context.Context) error {
	// TODO: Implement Method
	return nil
}

func (n *Node) ConfigureAsInternetGateway(ctx context.Context) error {
	// TODO: Implement Method
	return nil
}

func (n *Node) RemoveInternetGatewayConfig(ctx context.Context) error {
	// TODO: Implement Method
	return nil
}
