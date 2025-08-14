package schema

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/servercfg"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Node struct {
	ID            string `gorm:"primaryKey"`
	OwnerID       string
	HostID        string
	Host          Host `gorm:"-"`
	LocalAddress  string
	NetworkID     string
	Network       Network `gorm:"-"`
	NetworkRange  string
	NetworkRange6 string
	Address       string
	Address6      string
	Server        string
	Connected     bool
	Action        string

	// GatewayNodeID is the ID of the node that this node uses as a
	// Gateway. If nil, this node does not use any node as its
	// Gateway.
	GatewayNodeID *string
	// GatewayNode is the node that this node uses as a Gateway.
	// If nil, this node does not use any node as its Gateway.
	GatewayNode *Node `gorm:"foreignKey:GatewayNodeID"`
	// GatewayFor is the list of Nodes that use this node as a
	// Gateway.
	GatewayFor []Node `gorm:"foreignKey:GatewayNodeID;constraint:OnDelete:SET NULL;"`
	// GatewayNodeConfig is the Gateway configuration of this
	// node. If nil, this node is not a Gateway node.
	GatewayNodeConfig *datatypes.JSONType[GatewayNodeConfig] `gorm:"foreignKey:GatewayNodeConfigID"`

	// FailOverNodeID is the ID of the node that can be used to
	// connect to this node.
	FailOverNodeID *string
	// FailOverPeers is the list of peer nodes that this node
	// connects to using the network's FailOver.
	FailOverPeers datatypes.JSONMap
	// IsFailOver indicates if the node is a FailOver node in the
	// network.
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
	InternetGatewayFor []Node `gorm:"foreignKey:InternetGatewayNodeID;constraint:OnDelete:SET NULL;"`
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

type RangeWithMetric struct {
	Range  string `json:"range"`
	Metric uint32 `json:"metric"`
}

func (n *Node) TableName() string {
	return "nodes_v1"
}

func (n *Node) Create(ctx context.Context) error {
	dbctx := db.BeginTx(ctx)
	commit := false
	defer func() {
		if commit {
			db.CommitTx(dbctx)
		} else {
			db.RollbackTx(dbctx)
		}
	}()

	err := db.FromContext(dbctx).Model(n).Create(n).Error
	if err != nil {
		return err
	}

	err = n.updateRelations(dbctx)
	if err != nil {
		return err
	}

	commit = true
	return nil
}

func (n *Node) Get(ctx context.Context) error {
	dbctx := db.BeginTx(ctx)
	commit := false
	defer func() {
		if commit {
			db.CommitTx(dbctx)
		} else {
			db.RollbackTx(dbctx)
		}
	}()

	err := db.FromContext(dbctx).Model(n).
		Where("id = ?", n.ID).
		First(n).
		Error
	if err != nil {
		return err
	}

	err = n.fetchRelations(dbctx)
	if err != nil {
		return err
	}

	commit = true
	return nil
}

func (n *Node) GetHost(ctx context.Context) error {
	return db.FromContext(ctx).
		Raw(`
			SELECT hosts_v1.*
			FROM hosts_v1
			JOIN nodes_v1 ON hosts_v1.id = nodes_v1.host_id
			WHERE nodes_v1.id = ?
		`, n.ID).
		Scan(&n.Host).
		Error
}

func (n *Node) GetNetwork(ctx context.Context) error {
	return db.FromContext(ctx).
		Raw(`
			SELECT networks_v1.*
			FROM networks_v1
			JOIN nodes_v1 ON networks_v1.id = nodes_v1.network_id
			WHERE nodes_v1.id = ?
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
		"SELECT EXISTS (SELECT 1 FROM nodes_v1 WHERE id = ?)",
		n.ID,
	).Scan(&exists).Error
	return exists, err
}

func (n *Node) ExistsWithNetworkAndIPv4(ctx context.Context) (bool, error) {
	var exists bool
	err := db.FromContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM nodes_v1 WHERE network_id = ? AND address = ?)",
		n.NetworkID,
		n.Address,
	).Scan(&exists).Error
	return exists, err
}

func (n *Node) ExistsWithNetworkAndIPv6(ctx context.Context) (bool, error) {
	var exists bool
	err := db.FromContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM nodes_v1 WHERE network_id = ? AND address6 = ?)",
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
		SELECT hosts_v1.os, COUNT(nodes_v1.id) as count
		FROM hosts_v1 LEFT JOIN nodes_v1 ON hosts_v1.id = nodes_v1.host_id
		GROUP BY hosts_v1.os
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
	dbctx := db.BeginTx(ctx)
	commit := false
	defer func() {
		if commit {
			db.CommitTx(dbctx)
		} else {
			db.RollbackTx(dbctx)
		}
	}()

	err := db.FromContext(dbctx).Model(n).Save(n).Error
	if err != nil {
		return err
	}

	err = n.updateRelations(dbctx)
	if err != nil {
		return err
	}

	commit = true
	return nil
}

func (n *Node) UpdateFailOverPeers(ctx context.Context) error {
	return db.FromContext(ctx).Model(n).
		Update("fail_over_peers", n.FailOverPeers).
		Error
}

func (n *Node) UpdateLastCheckIn(ctx context.Context) error {
	return db.FromContext(ctx).Model(n).Update("last_check_in", n.LastCheckIn).Error
}

func (n *Node) Upsert(ctx context.Context) error {
	dbctx := db.BeginTx(ctx)
	commit := false
	defer func() {
		if commit {
			db.CommitTx(dbctx)
		} else {
			db.RollbackTx(dbctx)
		}
	}()

	err := db.FromContext(dbctx).Model(n).
		First(&Node{
			ID: n.ID,
		}).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = n.Create(dbctx)
			if err != nil {
				return err
			}
			commit = true
		} else {
			return err
		}
	} else {
		err = n.Update(dbctx)
		if err != nil {
			return err
		}
		commit = true
	}

	return nil
}

func (n *Node) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(n).Delete(n).Error
}

func (n *Node) IsEligibleToBeFailOver(ctx context.Context) (bool, error) {
	var isEligible bool
	err := db.FromContext(ctx).Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM nodes_v1
			JOIN networks_v1 ON nodes_v1.network_id = networks_v1.id
			JOIN hosts_v1 ON nodes_v1.host_id = hosts_v1.id
			WHERE nodes_v1.id = ? AND nodes_v1.gateway_node_id = ? AND networks_v1.fail_over_id = ? AND hosts_v1.os = ?
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

func (n *Node) fetchRelations(ctx context.Context) error {
	dbctx := db.BeginTx(ctx)
	commit := false
	defer func() {
		if commit {
			db.CommitTx(dbctx)
		} else {
			db.RollbackTx(dbctx)
		}
	}()

	err := db.FromContext(dbctx).Model(&Node{}).
		Select("id").
		Where("gateway_node_id = ?", n.ID).
		Find(&n.GatewayFor).
		Error
	if err != nil {
		return err
	}

	err = db.FromContext(dbctx).Model(&Node{}).
		Select("id").
		Where("internet_gateway_node_id = ?", n.ID).
		Find(&n.InternetGatewayFor).
		Error
	if err != nil {
		return err
	}

	commit = true
	return nil
}

func (n *Node) updateRelations(ctx context.Context) error {
	dbctx := db.BeginTx(ctx)
	commit := false
	defer func() {
		if commit {
			db.CommitTx(dbctx)
		} else {
			db.RollbackTx(dbctx)
		}
	}()

	gatewayClients := make([]string, len(n.GatewayFor))
	for i, gatewayClient := range n.GatewayFor {
		gatewayClients[i] = gatewayClient.ID
	}

	err := db.FromContext(dbctx).Model(&Node{}).
		Where("id IN ?", gatewayClients).
		Update("gateway_node_id", n.ID).
		Error
	if err != nil {
		return err
	}

	internetGatewayClients := make([]string, len(n.InternetGatewayFor))
	for i, internetGatewayClient := range n.InternetGatewayFor {
		internetGatewayClients[i] = internetGatewayClient.ID
	}

	return db.FromContext(dbctx).Model(&Node{}).
		Where("id IN ?", internetGatewayClients).
		Update("internet_gateway_node_id", n.ID).
		Error
}
