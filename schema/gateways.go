package schema

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

const gatewayTable = "gateways_v1"

type Gateway struct {
	ID                  string `gorm:"primaryKey"`
	NetworkID           string
	Range               string
	Range6              string
	PersistentKeepalive int32
	MTU                 int32
	IsAutoRelay         bool
	IsInternetGateway   bool
	RelayedNodes        datatypes.JSONMap
	AutoRelayedNodes    datatypes.JSONMap
	CreatedBy           string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (g *Gateway) TableName() string {
	return gatewayTable
}

func (g *Gateway) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Gateway{}).Create(g).Error
}

func (g *Gateway) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Gateway{}).Where("id = ?", g.ID).First(g).Error
}

func (g *Gateway) Update(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Gateway{}).Where("id = ?", g.ID).Updates(g).Error
}

func (g *Gateway) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Gateway{}).Where("id = ?", g.ID).Delete(g).Error
}

func (g *Gateway) ListByNetwork(ctx context.Context) ([]Gateway, error) {
	var ingresses []Gateway
	err := db.FromContext(ctx).Model(&Gateway{}).Where("network_id = ?", g.NetworkID).Find(&ingresses).Error
	return ingresses, err
}
