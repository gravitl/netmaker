package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/orchestrator/extensions"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

type GatewayOrchestrator struct {
	gwExt extensions.GatewayExtensions
}

func (g *GatewayOrchestrator) CreateGateway(ctx context.Context, node *schema.Node) (*schema.Gateway, error) {
	if node.Host.OS != "linux" {
		return nil, errors.New("gateway can only be created on linux based node")
	}

	if node.GatewayID.Valid {
		return nil, errors.New("node is already a gateway")
	}

	if node.RelayingNodeID.Valid {
		return nil, errors.New("gateway cannot be created on a relayed node")
	}

	gateway := &schema.Gateway{
		ID:                node.ID,
		NetworkID:         node.NetworkID,
		Range:             node.Network.AddressRange,
		Range6:            node.Network.AddressRange6,
		IsAutoRelay:       false,
		IsInternetGateway: false,
		RelayedNodes:      make(datatypes.JSONMap),
	}

	g.gwExt.ConfigureAutoRelay(gateway)

	err := gateway.Create(ctx)
	if err != nil {
		return nil, err
	}

	node.Tags[fmt.Sprintf("%s.%s", node.NetworkID, models.GwTagName)] = struct{}{}
	err = node.UpdateTags(ctx)
	if err != nil {
		return nil, err
	}

	node.Network.NodesUpdatedAt = time.Now()
	err = node.Network.UpdateNodesUpdatedAt(ctx)
	if err != nil {
		return nil, err
	}

	node.GatewayID = datatypes.NewNull(gateway.ID)
	node.Gateway = gateway
	return gateway, nil
}
