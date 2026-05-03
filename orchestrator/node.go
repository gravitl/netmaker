package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/orchestrator/extensions"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type NodeOrchestrator struct {
	nodeExt extensions.NodeExtensions
}

type NodeOrchestratorOptions struct {
	useKey                bool
	key                   *models.EnrollmentKey
	skipPublishPeerUpdate bool
}

type NodeOrchestratorOption func(options *NodeOrchestratorOptions) *NodeOrchestratorOptions

func UseKey(key *models.EnrollmentKey) NodeOrchestratorOption {
	return func(o *NodeOrchestratorOptions) *NodeOrchestratorOptions {
		o.useKey = true
		o.key = key
		return o
	}
}

func SkipPublishPeerUpdate() NodeOrchestratorOption {
	return func(o *NodeOrchestratorOptions) *NodeOrchestratorOptions {
		o.skipPublishPeerUpdate = true
		return o
	}
}

func (n *NodeOrchestrator) CreateNode(ctx context.Context, host *schema.Host, network *schema.Network, options ...NodeOrchestratorOption) (*schema.Node, error) {
	var ops NodeOrchestratorOptions
	for _, option := range options {
		option(&ops)
	}

	node := &schema.Node{
		ID:                 uuid.NewString(),
		HostID:             host.ID.String(),
		Host:               host,
		NetworkID:          network.ID,
		Network:            network,
		Connected:          true,
		LastCheckIn:        time.Now(),
		ExpirationDateTime: time.Now().AddDate(100, 1, 0),
		AutoRelayedPeers:   datatypes.NewJSONType(make(map[string]string)),
		Tags:               make(datatypes.JSONMap),
	}

	if ops.useKey {
		n.nodeExt.ConfigureAutoAssignGateway(node, ops.key)

		for _, tag := range ops.key.Groups {
			node.Tags[string(tag)] = true
		}
	}

	// TODO: Ensure concurrency safe ip allocation.
	if network.AddressRange != "" {
		ip, err := GetRepository().NetworkOrchestrator().AllocateNodeIP(ctx, network)
		if err != nil {
			return nil, err
		}
		_, cidr, err := net.ParseCIDR(network.AddressRange)
		if err != nil {
			return nil, err
		}
		cidr.IP = ip
		node.Address = cidr.String()
	}

	if network.AddressRange6 != "" {
		ip, err := GetRepository().NetworkOrchestrator().AllocateNodeIPv6(ctx, network)
		if err != nil {
			return nil, err
		}
		_, cidr, err := net.ParseCIDR(network.AddressRange6)
		if err != nil {
			return nil, err
		}
		cidr.IP = ip
		node.Address6 = cidr.String()
	}

	err := node.Create(ctx)
	if err != nil {
		return nil, err
	}

	host.Nodes = append(host.Nodes, node.ID)
	err = host.Upsert(ctx)
	if err != nil {
		return nil, err
	}

	go logic.CheckZombies(node)

	go func() {
		err := logic.UpdateMetrics(node.ID, &models.Metrics{Connectivity: make(map[string]models.Metric)})
		if err != nil {
			logger.Log(1, fmt.Sprintf("failed to initialize metrics for node (%s): %v", node.ID, err))
		}
	}()

	if host.IsDefault {
		err = n.CreateGateway(ctx, node)
		if err != nil {
			return nil, err
		}
	} else if ops.useKey && ops.key.Relay != uuid.Nil {
		gateway := &schema.Node{
			ID: ops.key.Relay.String(),
		}
		err = gateway.Get(ctx)
		if err == nil {
			node.RelayingNodeID = datatypes.NewNull(ops.key.Relay.String())
			err = node.UpdateRelayingNode(ctx)
			if err != nil {
				return nil, err
			}

			gateway.RelayedClients[node.ID] = struct{}{}
			err = gateway.UpdateRelayedClients(ctx)
			if err != nil {
				return nil, err
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	action := models.JoinHostToNetwork
	if len(host.Nodes) == 1 {
		action = models.RequestPull
	}

	// TODO: figure out mq placement.
	go func() {
		if err := mq.HostUpdate(&models.HostUpdate{
			Action: action,
			Host:   *host,
			Node:   *logic.ConvertSchemaNodeToModelsNode(node),
		}); err != nil {
			logger.Log(1, "failed to send host update for node", node.ID, err.Error())
		}
	}()

	if !ops.skipPublishPeerUpdate {
		go func() {
			if err := mq.PublishPeerUpdate(false); err != nil {
				logger.Log(1, "failed to publish peer update for node", node.ID, err.Error())
			}
		}()
	}

	return node, nil
}

func (n *NodeOrchestrator) CreateGateway(ctx context.Context, node *schema.Node) error {
	if node.Host.OS != "linux" {
		return errors.New("gateway can only be created on linux based node")
	}

	if node.IsGateway {
		return errors.New("node is already a gateway")
	}

	if node.RelayingNodeID.Valid {
		return errors.New("gateway cannot be created on a relayed node")
	}

	node.IsGateway = true
	node.AllowRelayingAllTraffic = false

	n.nodeExt.ConfigureAutoRelay(node)

	err := node.Update(ctx)
	if err != nil {
		return err
	}

	node.Tags[fmt.Sprintf("%s.%s", node.NetworkID, models.GwTagName)] = struct{}{}
	err = node.UpdateTags(ctx)
	if err != nil {
		return err
	}

	node.Network.NodesUpdatedAt = time.Now()
	return node.Network.UpdateNodesUpdatedAt(ctx)
}
