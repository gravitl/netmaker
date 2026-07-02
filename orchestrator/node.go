package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/orchestrator/extensions"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
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
		Status:             schema.OnlineSt,
		IsAutoRelay:        "no",
		RelayedClients:     make(datatypes.JSONMap),
		RelayedIGWClients:  make(datatypes.JSONMap),
		AutoRelayedPeers:   datatypes.NewJSONType(make(map[string]string)),
		Tags:               make(datatypes.JSONMap),
		LastCheckIn:        time.Now(),
		ExpirationDateTime: time.Now().AddDate(100, 1, 0),
	}

	if ops.useKey {
		n.nodeExt.ConfigureAutoAssignGateway(node, ops.key)

		for _, tag := range ops.key.Tags {
			n.nodeExt.ConfigureTag(node, models.TagID(tag))
		}
	}

	networkOrch := GetRepository().NetworkOrchestrator()
	var reservedIPv4, reservedIPv6 string

	if network.AddressRange != "" {
		ip, err := networkOrch.AllocateNodeIP(ctx, network)
		if err != nil {
			return nil, err
		}
		reservedIPv4 = ip.String()
		_, cidr, err := net.ParseCIDR(network.AddressRange)
		if err != nil {
			networkOrch.FreeIPv4Reservation(network.ID, reservedIPv4)
			return nil, err
		}
		cidr.IP = ip
		node.Address = cidr.String()
	}

	if network.AddressRange6 != "" {
		ip, err := networkOrch.AllocateNodeIPv6(ctx, network)
		if err != nil {
			if reservedIPv4 != "" {
				networkOrch.FreeIPv4Reservation(network.ID, reservedIPv4)
			}
			return nil, err
		}
		reservedIPv6 = ip.String()
		_, cidr, err := net.ParseCIDR(network.AddressRange6)
		if err != nil {
			networkOrch.FreeIPv4Reservation(network.ID, reservedIPv4)
			networkOrch.FreeIPv6Reservation(network.ID, reservedIPv6)
			return nil, err
		}
		cidr.IP = ip
		node.Address6 = cidr.String()
	}

	if node.TenantID == "" {
		node.TenantID = scope.ID(logic.DefaultScope(ctx))
	}
	err := node.Create(ctx)
	// Reservations are freed regardless of outcome: on success the DB is authoritative,
	// on failure the IPs must be available for reallocation.
	if reservedIPv4 != "" {
		networkOrch.FreeIPv4Reservation(network.ID, reservedIPv4)
	}
	if reservedIPv6 != "" {
		networkOrch.FreeIPv6Reservation(network.ID, reservedIPv6)
	}
	if err != nil {
		return nil, err
	}

	host.Nodes = append(host.Nodes, node.ID)
	if host.TenantID == "" {
		host.TenantID = scope.ID(logic.DefaultScope(ctx))
	}
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
	} else if ops.useKey && ops.key.GatewayID != nil {
		gateway := &schema.Node{
			ID: *ops.key.GatewayID,
		}
		err = gateway.Get(ctx)
		if err == nil {
			// TODO: merge operation
			relayID := *ops.key.GatewayID
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

	go func() {
		modelsNode := logic.ConvertSchemaNodeToModelsNode(node)

		modelsNode.PostureChecksViolations, modelsNode.PostureCheckViolationSeverityLevel = logic.CheckPostureViolations(logic.GetPostureCheckDeviceInfoByNode(modelsNode), schema.NetworkID(node.Network.Name))
		node.PostureCheckSeverity = modelsNode.PostureCheckViolationSeverityLevel
		node.PostureCheckLastEvaluationCycleID = uuid.NewString()
		node.PostureCheckLastEvaluatedAt = time.Now().UTC()

		_violations := make([]schema.PostureCheckViolation, 0, len(modelsNode.PostureChecksViolations))
		for _, violation := range modelsNode.PostureChecksViolations {
			_violations = append(_violations, schema.PostureCheckViolation{
				EvaluationCycleID: node.PostureCheckLastEvaluationCycleID,
				CheckID:           violation.CheckID,
				NodeID:            node.ID,
				Name:              violation.Name,
				Attribute:         violation.Attribute,
				Message:           violation.Message,
				Severity:          violation.Severity,
				EvaluatedAt:       node.PostureCheckLastEvaluatedAt,
			})
		}
		err = node.UpsertViolations(db.WithContext(context.TODO()), _violations)
		if err != nil {
			logger.Log(1, fmt.Sprintf("failed to upsert node (%s) posture check violations: %v", modelsNode.ID, err))
		}

		if !ops.skipHostUpdate {
			action := models.JoinHostToNetwork
			if len(host.Nodes) == 1 {
				action = models.RequestPull
			}

			err := mq.HostUpdate(&models.HostUpdate{
				Action: action,
				Host:   *host,
				Node:   *modelsNode,
			})
			if err != nil {
				logger.Log(1, "failed to send host update for node", node.ID, err.Error())
			}
		}

		if !ops.skipPublishPeerUpdate {
			err := mq.PublishPeerUpdate(false)
			if err != nil {
				logger.Log(1, "failed to publish peer update for node", node.ID, err.Error())
			}
			time.Sleep(time.Second * 30)
			logic.TriggerCollectMetrics(host.ID.String(), node.ID, "join")
		}
	}()

	return node, nil
}

func (n *NodeOrchestrator) CreateGateway(ctx context.Context, node *schema.Node, options ...Option) error {
	ops := applyOptions(options...)

	node.IsGateway = true

	if ops.isInternetGateway {
		node.Host.DNS = "yes"
		node.Host.IsStaticPort = true
		if node.Host.TenantID == "" {
			node.Host.TenantID = scope.ID(logic.DefaultScope(ctx))
		}
		err := node.Host.Upsert(ctx)
		if err != nil {
			return err
		}

		node.IsInternetGateway = true
	}

	n.nodeExt.ConfigureAutoRelay(node)

	node.Tags[fmt.Sprintf("%s.%s", node.Network.Name, models.GwTagName)] = struct{}{}

	err := node.Update(ctx)
	if err != nil {
		return err
	}

	for _, relayedClientID := range ops.relayedClients {
		node.RelayedClients[relayedClientID] = struct{}{}
	}

	if ops.isInternetGateway {
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
			node.RelayedIGWClients[igwClientID] = struct{}{}

			if igwClient.AutoAssignGateway {
				err = igwClient.ResetAutoAssignGateway(ctx)
				if err != nil {
					return err
				}
			}

			igwClient.IsIGWClient = true
			igwClient.RelayedByNodeID = &nodeID

			err = igwClient.AssignGateway(ctx)
			if err != nil {
				return err
			}
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

	if !ops.skipNodeUpdate {
		go func() {
			err := mq.NodeUpdate(logic.ConvertSchemaNodeToModelsNode(node))
			if err != nil {
				logger.Log(1, "failed to send node update for node", node.ID, err.Error())
			}
		}()
	}

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

			if igwClient.IsAutoRelay == "yes" {
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
