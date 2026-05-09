package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	dbtypes "github.com/gravitl/netmaker/db/types"
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

func (n *NodeOrchestrator) CreateNode(ctx context.Context, host *schema.Host, network *schema.Network, options ...Option) (*schema.Node, error) {
	ops := applyOptions(options...)

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
		err = n.ValidateCreateGateway(ctx, node, SkipPublishPeerUpdate())
		if err != nil {
			return nil, err
		}

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
			relayID := ops.key.Relay.String()
			node.RelayedByNodeID = &relayID
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

func (n *NodeOrchestrator) CreateGateway(ctx context.Context, node *schema.Node, options ...Option) error {
	ops := applyOptions(options...)

	node.IsGateway = true

	if ops.isInternetGateway {
		node.Host.DNS = "yes"
		node.Host.IsStaticPort = true
		err := node.Host.Upsert(ctx)
		if err != nil {
			return err
		}

		node.IsInternetGateway = true
	}

	n.nodeExt.ConfigureAutoRelay(node)

	node.Tags[fmt.Sprintf("%s.%s", node.NetworkID, models.GwTagName)] = struct{}{}

	err := node.Update(ctx)
	if err != nil {
		return err
	}

	for _, relayedClientID := range ops.relayedClients {
		node.RelayedClients[relayedClientID] = struct{}{}
	}

	nodeID := node.ID
	for _, igwClientID := range ops.igwClients {
		igwClient := &schema.Node{
			ID: igwClientID,
		}
		err = igwClient.Get(ctx)
		if err != nil {
			return err
		}

		node.RelayedClients[igwClientID] = struct{}{}

		if igwClient.AutoAssignGateway {
			err = igwClient.ResetAutoAssignGateway(ctx)
			if err != nil {
				return err
			}
		}

		igwClient.IsIGWClient = true
		igwClient.RelayedByNodeID = &nodeID

		err = igwClient.AssignInternetGateway(ctx)
		if err != nil {
			return err
		}
	}

	err = node.SetRelayedClients(ctx)
	if err != nil {
		return err
	}

	err = node.ResetAutoRelayedPeers(ctx)
	if err != nil {
		return err
	}

	for relayedClientID := range node.RelayedClients {
		err = (&schema.Node{
			ID:        relayedClientID,
			NetworkID: node.NetworkID,
		}).ResetAutoRelayedPeers(ctx)
		if err != nil {
			return err
		}
	}

	node.Network.NodesUpdatedAt = time.Now()
	err = node.Network.UpdateNodesUpdatedAt(ctx)
	if err != nil {
		return err
	}

	go func() {
		err := mq.NodeUpdate(logic.ConvertSchemaNodeToModelsNode(node))
		if err != nil {
			logger.Log(1, "failed to send node update for node", node.ID, err.Error())
		}
	}()

	if !ops.skipPublishPeerUpdate {
		go func() {
			err := mq.PublishPeerUpdate(false)
			if err != nil {
				logger.Log(1, "failed to publish peer update for node", node.ID, err.Error())
			}
		}()
	}

	return nil
}

func (n *NodeOrchestrator) ValidateCreateGateway(ctx context.Context, node *schema.Node, options ...Option) error {
	ops := applyOptions(options...)

	if node.Host.OS != "linux" {
		return fmt.Errorf("gateway can only be created on linux based node")
	}

	if node.AutoAssignGateway {
		return fmt.Errorf("cannot set node %s as gateway while AutoAssignGateway is enabled", node.Host.Name)
	}

	if node.IsGateway {
		return fmt.Errorf("node %s is already a gateway", node.Host.Name)
	}

	if node.RelayedByNodeID != nil {
		return fmt.Errorf("relayed node %s cannot be used as a gateway", node.Host.Name)
	}

	for _, relayedClientID := range ops.relayedClients {
		err := (&schema.Node{
			ID: relayedClientID,
		}).Get(ctx)
		if err != nil {
			return err
		}
	}

	if ops.isInternetGateway {
		if node.Host.FirewallInUse == schema.FIREWALL_NONE {
			return fmt.Errorf("host must have iptables or nftables installed")
		}

		if node.IsIGWClient {
			return fmt.Errorf("node %s is using a internet gateway already", node.Host.Name)
		}

		if node.RelayedByNodeID != nil {
			return fmt.Errorf("node %s is being relayed", node.Host.Name)
		}

		for _, igwClientID := range ops.igwClients {
			igwClient := &schema.Node{
				ID: igwClientID,
			}
			err := igwClient.Get(ctx, dbtypes.WithPreloads("Host"))
			if err != nil {
				return err
			}

			if igwClient.Host.IsDefault {
				return fmt.Errorf("default host %s cannot be set to use internet gateway", igwClient.Host.Name)
			}

			if igwClient.IsAutoRelay {
				return fmt.Errorf("node %s acting as auto relay cannot use internet gateway", igwClient.Host.Name)
			}

			if igwClient.IsGateway {
				return fmt.Errorf("node %s acting as gateway cannot use internet gateway", igwClient.Host.Name)
			}

			if igwClient.IsInternetGateway {
				return fmt.Errorf("node %s acting as internet gateway cannot use another internet gateway", igwClient.Host.Name)
			}

			if igwClient.IsIGWClient {
				return fmt.Errorf("node %s is already using a internet gateway", igwClient.Host.Name)
			}

			if igwClient.RelayedByNodeID != nil && *igwClient.RelayedByNodeID != node.ID {
				return fmt.Errorf("node %s is already being relayed", igwClient.Host.Name)
			}

			otherNodes, err := (&schema.Node{}).ListAll(
				ctx,
				dbtypes.WithFilter("host_id", igwClient.HostID),
				dbtypes.WithNotFilter("id", igwClient.ID),
			)
			if err != nil {
				return err
			}

			for _, otherNode := range otherNodes {
				if otherNode.IsIGWClient && otherNode.RelayedByNodeID != nil {
					otherNodeIGW := &schema.Node{
						ID: *otherNode.RelayedByNodeID,
					}
					err = otherNodeIGW.Get(ctx)
					if err != nil {
						return err
					}

					if otherNodeIGW.HostID != node.HostID {
						return errors.New("nodes on same host cannot use different internet gateway")
					}
				}
			}
		}
	}

	return nil
}
