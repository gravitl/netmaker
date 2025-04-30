package schema

import (
	"context"
	"database/sql"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/servercfg"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Node struct {
	ID      string `gorm:"primaryKey"`
	OwnerID string
	HostID  string
	// Ideally, a foreign key relationship between host and node
	// must exist, but here we tell gorm to not create the
	// constraint.
	//
	// 1. A host's lifecycle may differ from a node's lifecycle.
	// So we don't want to cascade delete a node record when the
	// host is deleted.
	//
	// 2. Since we don't allow updating a host's id, we don't
	// need to cascade update the foreign key.
	Host         Host `gorm:"-"`
	LocalAddress string
	NetworkID    string
	// Network foreign key relationship is also ignored for the
	// same reason as Host.
	Network       Network `gorm:"-"`
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

	// FailOverPeers is the list of peer nodes that this node
	// connects to using the network's FailOver.
	FailOverPeers datatypes.JSONMap

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
	return db.FromContext(ctx).Model(n).First(n).Error
}

func (n *Node) GetHost(ctx context.Context) error {
	return db.FromContext(ctx).
		Raw(`
			SELECT hosts.*
			FROM hosts
			JOIN nodes ON hosts.id = nodes.host_id
			WHERE nodes.id = ?
		`, n.ID).
		Scan(&n.Host).
		Error
}

func (n *Node) GetNetwork(ctx context.Context) error {
	return db.FromContext(ctx).
		Raw(`
			SELECT networks.*
			FROM networks
			JOIN nodes ON networks.id = nodes.network_id
			WHERE nodes.id = ?
		`, n.ID).
		Scan(&n.Network).
		Error
}

func (n *Node) GetByHostIDAndNetworkID(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("host_id = ? AND network_id = ?", n.HostID, n.NetworkID).
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

func (n *Node) ListZombies(ctx context.Context) ([]Node, error) {
	var nodes []Node
	err := db.FromContext(ctx).Model(&Node{}).
		Where("id <> ? AND host_id = ? AND network_id = ?", n.ID, n.HostID, n.NetworkID).
		Find(&nodes).
		Error
	return nodes, err
}

func (n *Node) ListExpired(ctx context.Context) ([]Node, error) {
	var nodes []Node
	err := db.FromContext(ctx).Model(&Node{}).
		Where("expiration_date_time < ?", time.Now()).
		First(&nodes).
		Error
	return nodes, err
}

func (n *Node) Exists(ctx context.Context) (bool, error) {
	var exists bool
	err := db.FromContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM nodes WHERE id = ?)",
		n.ID,
	).Scan(&exists).Error
	return exists, err
}

func (n *Node) ExistsWithNetworkAndIPv4(ctx context.Context) (bool, error) {
	var exists bool
	err := db.FromContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM nodes WHERE network_id = ? AND address = ?)",
		n.NetworkID,
		n.Address,
	).Scan(&exists).Error
	return exists, err
}

func (n *Node) ExistsWithNetworkAndIPv6(ctx context.Context) (bool, error) {
	var exists bool
	err := db.FromContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM nodes WHERE network_id = ? AND address6 = ?)",
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
	return db.FromContext(ctx).Model(n).Updates(n).Error
}

func (n *Node) UpdateFailOverPeers(ctx context.Context) error {
	return db.FromContext(ctx).Model(n).Update("fail_over_peers", n.FailOverPeers).Error
}

func (n *Node) UpdateLastCheckIn(ctx context.Context) error {
	return db.FromContext(ctx).Model(n).Update("last_check_in", n.LastCheckIn).Error
}

func (n *Node) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Save(n).Error
}

func (n *Node) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(n).Delete(n).Error
}

func (n *Node) IsEligibleToBeFailOver(ctx context.Context) (bool, error) {
	var isEligible bool
	err := db.FromContext(ctx).Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM nodes
			JOIN networks ON nodes.network_id = networks.id
			JOIN hosts ON nodes.host_id = hosts.id
			WHERE nodes.id = ? AND nodes.gateway_node_id = ? AND networks.fail_over_id = ? AND hosts.os = ?
		)`,
		n.ID,
		nil,
		nil,
		"linux",
	).Scan(&isEligible).Error
	return isEligible, err
}

func (n *Node) ResetAndRemoveFromFailOverPeers(ctx context.Context) error {
	return db.FromContext(ctx).Transaction(func(tx *gorm.DB) error {
		switch servercfg.GetDB() {
		case "sqlite":
			path := "$." + n.ID
			err := tx.Model(&Node{}).
				Where("network_id = ?", n.NetworkID).
				Update("fail_over_peers", gorm.Expr("json_remove(fail_over_peers, ?)", path)).
				Error
			if err != nil {
				return err
			}
		case "postgres":
			err := tx.Model(&Node{}).
				Where("network_id = ?", n.NetworkID).
				Update("fail_over_peers", gorm.Expr("fail_over_peers - ?", n.ID)).
				Error
			if err != nil {
				return err
			}
		}

		return tx.Model(n).
			Update("fail_over_peers", datatypes.JSONMap{}).
			Error
	})
}
