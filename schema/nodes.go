package schema

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"gorm.io/datatypes"
)

const nodesTable = "nodes_v1"

// TODO: check network and host delete cascade issues.

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
	GatewayID                         *string
	Gateway                           *Gateway `gorm:"foreignKey:GatewayID;constraint:OnDelete:SET NULL"`
	RelayGatewayID                    *string
	RelayGateway                      *Gateway `gorm:"foreignKey:RelayGatewayID;constraint:OnDelete:SET NULL"`
	Tags                              datatypes.JSONMap
	PostureCheckStatus                string
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

func (n *Node) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).Where("id = ?", n.ID).First(n).Error
}

func (n *Node) GetByHostAndNetwork(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("host_id = ? AND network = ?", n.HostID, n.Network).
		First(n).Error
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
	err := db.FromContext(ctx).Model(&Node{}).Where("network = ?", n.Network).Find(&nodes).Error
	return nodes, err
}

func (n *Node) ListByHost(ctx context.Context) ([]Node, error) {
	var nodes []Node
	err := db.FromContext(ctx).Model(&Node{}).Where("host_id = ?", n.HostID).Find(&nodes).Error
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
		tx.Rollback()
		return err
	}

	if len(violations) > 0 {
		err := tx.Create(&violations).Error
		if err != nil {
			tx.Rollback()
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
