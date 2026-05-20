package migrate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func migrateV1_5_2(ctx context.Context) error {
	err := migratePendingUsers(ctx)
	if err != nil {
		return err
	}

	err = migrateUserInvites(ctx)
	if err != nil {
		return err
	}

	return migrateNodes(ctx)
}

func migratePendingUsers(ctx context.Context) error {
	records, err := kvList(ctx, database.PENDING_USERS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}

	for _, record := range records {
		var pendingUser models.User
		err = json.Unmarshal([]byte(record), &pendingUser)
		if err != nil {
			return err
		}

		_pendingUser := &schema.PendingUser{
			Username:                   pendingUser.UserName,
			ExternalIdentityProviderID: pendingUser.ExternalIdentityProviderID,
		}

		logger.Log(4, fmt.Sprintf("migrating pending user %s", _pendingUser.Username))

		err = _pendingUser.Create(ctx)
		if err != nil {
			logger.Log(4, fmt.Sprintf("migrating pending user %s failed: %v", _pendingUser.Username, err))
			return err
		}
	}

	return nil
}

func migrateUserInvites(ctx context.Context) error {
	records, err := kvList(ctx, database.USER_INVITES_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}

	for _, record := range records {
		var userInvite models.UserInvite
		err = json.Unmarshal([]byte(record), &userInvite)
		if err != nil {
			return err
		}

		_userInvite := &schema.UserInvite{
			InviteCode:     userInvite.InviteCode,
			InviteURL:      userInvite.InviteURL,
			Email:          userInvite.Email,
			PlatformRoleID: userInvite.PlatformRoleID,
			UserGroups:     datatypes.NewJSONType(userInvite.UserGroups),
		}

		logger.Log(4, fmt.Sprintf("migrating user invite %s/%s", _userInvite.InviteCode, _userInvite.Email))

		err = _userInvite.Create(ctx)
		if err != nil {
			logger.Log(4, fmt.Sprintf("migrating user invite %s/%s failed: %v", _userInvite.InviteCode, _userInvite.Email, err))
			return err
		}
	}

	return nil
}

func migrateNodes(ctx context.Context) error {
	records, err := kvList(ctx, database.NODES_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}

	for _, record := range records {
		var node models.Node
		err = json.Unmarshal([]byte(record), &node)
		if err != nil {
			return err
		}

		var address, address6 string
		if node.Address.IP != nil {
			address = node.Address.String()
		}

		if node.Address6.IP != nil {
			address6 = node.Address6.String()
		}

		if node.ExpirationDateTime.IsZero() {
			node.ExpirationDateTime = time.Now().AddDate(100, 1, 0)
		}

		network := &schema.Network{
			Name: node.Network,
		}
		err = network.Get(ctx)
		if err != nil {
			return err
		}

		if node.AutoRelayedPeers == nil {
			node.AutoRelayedPeers = make(map[string]string)
		}

		relayedClients := make(datatypes.JSONMap)
		relayedIGWClients := make(datatypes.JSONMap)
		if node.IsIngressGateway || node.IsRelay || node.IsInternetGateway || node.IsFailOver {
			node.IsGw = true
			node.IsIngressGateway = true
			node.IsRelay = true
			node.IsAutoRelay = false
			for _, relayedNodeID := range node.RelayedNodes {
				relayedClients[relayedNodeID] = struct{}{}
			}
			for _, inetNodeClientID := range node.InetNodeReq.InetNodeClientIDs {
				relayedClients[inetNodeClientID] = struct{}{}
				relayedIGWClients[inetNodeClientID] = struct{}{}
			}
			if servercfg.IsPro {
				node.IsAutoRelay = true
				if node.Tags == nil {
					node.Tags = make(map[models.TagID]struct{})
				}
				node.Tags[models.TagID(fmt.Sprintf("%s.%s", node.Network, models.GwTagName))] = struct{}{}
				delete(node.Tags, models.TagID(fmt.Sprintf("%s.%s", node.Network, models.OldRemoteAccessTagName)))
			}
		}

		isAutoRelay := "no"
		if node.IsAutoRelay {
			isAutoRelay = "yes"
		}

		additionalEndpoints := make([]string, 0, len(node.AdditionalRagIps))
		for _, additionalEndpoint := range node.AdditionalRagIps {
			endpointString := additionalEndpoint.String()
			if endpointString != "<nil>" {
				additionalEndpoints = append(additionalEndpoints, endpointString)
			}
		}

		var relayedByNodeID *string
		var isIGWClient bool
		if !node.IsGw {
			if node.IsRelayed {
				relayedBy := node.RelayedBy
				relayedByNodeID = &relayedBy
			}
			if node.InternetGwID != "" {
				igwID := node.InternetGwID
				relayedByNodeID = &igwID
				isIGWClient = true
				node.AutoAssignGateway = false
			}
		}

		tags := make(datatypes.JSONMap)
		for tagID := range node.Tags {
			tags[tagID.String()] = struct{}{}
		}

		_node := &schema.Node{
			ID:                                node.ID.String(),
			HostID:                            node.HostID.String(),
			NetworkID:                         network.ID,
			Address:                           address,
			Address6:                          address6,
			Connected:                         node.Connected,
			Action:                            node.Action,
			Status:                            node.Status,
			PendingDelete:                     node.PendingDelete,
			AutoAssignGateway:                 node.AutoAssignGateway,
			IsGateway:                         node.IsGw,
			IsAutoRelay:                       isAutoRelay,
			IsInternetGateway:                 node.IsGw && node.IsInternetGateway,
			AdditionalGatewayEndpoints:        additionalEndpoints,
			RelayedClients:                    relayedClients,
			RelayedIGWClients:                 relayedIGWClients,
			RelayedByNodeID:                   relayedByNodeID,
			IsIGWClient:                       isIGWClient,
			AutoRelayedPeers:                  datatypes.NewJSONType(node.AutoRelayedPeers),
			Tags:                              tags,
			PostureCheckSeverity:              node.PostureCheckVolationSeverityLevel,
			PostureCheckLastEvaluationCycleID: node.LastEvaluatedAt.Format(time.RFC3339),
			Metadata:                          node.Metadata,
			LastCheckIn:                       node.LastCheckIn,
			ExpirationDateTime:                node.ExpirationDateTime,
			CreatedAt:                         node.LastModified,
			UpdatedAt:                         node.LastModified,
		}

		_node.Status = logic.GetNodeCheckInStatus(_node)

		logger.Log(4, fmt.Sprintf("migrating node %s", _node.ID))

		err = _node.Create(ctx)
		if err != nil {
			logger.Log(4, fmt.Sprintf("migrating node %s failed: %v", _node.ID, err))
			return err
		}

		logger.Log(4, fmt.Sprintf("migrating node %s egress", _node.ID))

		err = migrateNodes_Egress(ctx, &node)
		if err != nil {
			logger.Log(4, fmt.Sprintf("migrating node %s egress failed: %v", _node.ID, err))
			return err
		}

		logger.Log(4, fmt.Sprintf("migrating node %s nameserver", _node.ID))

		err = migrateNodes_Nameserver(ctx, &node)
		if err != nil {
			logger.Log(4, fmt.Sprintf("migrating node %s nameserver failed: %v", _node.ID, err))
			return err
		}

		logger.Log(4, fmt.Sprintf("migrating node %s violations", _node.ID))

		err = migrateNodes_PostureCheckViolations(ctx, &node, _node)
		if err != nil {
			logger.Log(4, fmt.Sprintf("migrating node %s violations failed: %v", _node.ID, err))
			return err
		}

	}

	logger.Log(4, "cleaning up nodes post migration")

	err = migrateNodes_CleanUp(ctx)
	if err != nil {
		logger.Log(4, fmt.Sprintf("post migration nodes clean up failed: %v", err))
		return err
	}

	return nil
}

func migrateNodes_Egress(ctx context.Context, node *models.Node) error {
	if node.IsEgressGateway {
		superAdmin := &schema.User{}
		err := superAdmin.GetSuperAdmin(ctx)
		if err != nil {
			return err
		}

		host := &schema.Host{
			ID: node.HostID,
		}
		err = host.Get(ctx)
		if err != nil {
			return err
		}

		egressRanges, update := removeInterGw(node.EgressGatewayRanges)
		if update {
			node.EgressGatewayRequest.Ranges = egressRanges
			node.EgressGatewayRanges = egressRanges
		}
		if len(node.EgressGatewayRequest.Ranges) > 0 && len(node.EgressGatewayRequest.RangesWithMetric) == 0 {
			for _, egressRangeI := range node.EgressGatewayRequest.Ranges {
				node.EgressGatewayRequest.RangesWithMetric = append(node.EgressGatewayRequest.RangesWithMetric, models.EgressRangeMetric{
					Network:     egressRangeI,
					RouteMetric: 256,
				})
			}
		}

		for _, rangeMetric := range node.EgressGatewayRequest.RangesWithMetric {
			egressCheck := &schema.Egress{
				Range: rangeMetric.Network,
			}
			err = egressCheck.DoesEgressRouteExists(ctx)
			if err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
			} else {
				egressCheck.Nodes[node.ID.String()] = rangeMetric.RouteMetric
				err = egressCheck.Update(ctx)
				if err != nil {
					return err
				}
			}

			egress := &schema.Egress{
				ID:          uuid.New().String(),
				Name:        fmt.Sprintf("%s egress", rangeMetric.Network),
				Description: "",
				Network:     node.Network,
				Nodes: datatypes.JSONMap{
					node.ID.String(): rangeMetric.RouteMetric,
				},
				Tags:      make(datatypes.JSONMap),
				Range:     rangeMetric.Network,
				Nat:       node.EgressGatewayRequest.NatEnabled == "yes",
				Status:    true,
				CreatedBy: superAdmin.Username,
				CreatedAt: time.Now().UTC(),
			}
			if !egress.Nat {
				egress.Mode = schema.DisabledNAT
			}
			err = egress.Create(ctx)
			if err != nil {
				return err
			}

			acl := models.Acl{
				ID:          uuid.New().String(),
				Name:        "egress node policy",
				MetaData:    "",
				Default:     false,
				ServiceType: models.Any,
				NetworkID:   schema.NetworkID(node.Network),
				Proto:       models.ALL,
				RuleType:    models.DevicePolicy,
				Src: []models.AclPolicyTag{

					{
						ID:    models.NodeTagID,
						Value: "*",
					},
				},
				Dst: []models.AclPolicyTag{
					{
						ID:    models.EgressID,
						Value: egress.ID,
					},
				},

				AllowedDirection: models.TrafficDirectionBi,
				Enabled:          true,
				CreatedBy:        "auto",
				CreatedAt:        time.Now().UTC(),
			}
			err = kvInsert(ctx, database.ACLS_TABLE_NAME, acl.ID, acl)
			if err != nil {
				return err
			}

			acl = models.Acl{
				ID:          uuid.New().String(),
				Name:        "egress node policy",
				MetaData:    "",
				Default:     false,
				ServiceType: models.Any,
				NetworkID:   schema.NetworkID(node.Network),
				Proto:       models.ALL,
				RuleType:    models.UserPolicy,
				Src: []models.AclPolicyTag{

					{
						ID:    models.UserAclID,
						Value: "*",
					},
				},
				Dst: []models.AclPolicyTag{
					{
						ID:    models.EgressID,
						Value: egress.ID,
					},
				},

				AllowedDirection: models.TrafficDirectionBi,
				Enabled:          true,
				CreatedBy:        "auto",
				CreatedAt:        time.Now().UTC(),
			}
			err = kvInsert(ctx, database.ACLS_TABLE_NAME, acl.ID, acl)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func migrateNodes_Nameserver(ctx context.Context, node *models.Node) error {
	if !node.IsGw {
		return nil
	}

	if node.IngressDNS != "" {
		superAdmin := &schema.User{}
		err := superAdmin.GetSuperAdmin(ctx)
		if err != nil {
			return err
		}

		var nsIPs []string
		for _, nsIP := range strings.Split(node.IngressDNS, ",") {
			nsIP = strings.TrimSpace(nsIP)

			if (node.Address.IP != nil && node.Address.IP.String() == nsIP) ||
				(node.Address6.IP != nil && node.Address6.IP.String() == nsIP) {
				continue
			}
			if nsIP == "8.8.8.8" || nsIP == "1.1.1.1" || nsIP == "9.9.9.9" {
				continue
			}

			nsIPs = append(nsIPs, nsIP)
		}

		if len(nsIPs) > 0 {
			host := &schema.Host{
				ID: node.HostID,
			}
			err = host.Get(ctx)
			if err != nil {
				return err
			}
			ns := schema.Nameserver{
				ID:        uuid.NewString(),
				Name:      fmt.Sprintf("%s gw nameservers", host.Name),
				NetworkID: node.Network,
				Servers:   nsIPs,
				MatchAll:  true,
				Domains: []schema.NameserverDomain{
					{
						Domain: ".",
					},
				},
				Nodes: datatypes.JSONMap{
					node.ID.String(): struct{}{},
				},
				Tags:      make(datatypes.JSONMap),
				Status:    true,
				CreatedBy: superAdmin.Username,
			}
			return ns.Create(ctx)
		}
	}

	return nil
}

func migrateNodes_PostureCheckViolations(ctx context.Context, node *models.Node, _node *schema.Node) error {
	if !node.LastEvaluatedAt.IsZero() {
		violations := make([]schema.PostureCheckViolation, 0, len(node.PostureChecksViolations))
		for _, violation := range node.PostureChecksViolations {
			violations = append(violations, schema.PostureCheckViolation{
				EvaluationCycleID: node.LastEvaluatedAt.Format(time.RFC3339),
				CheckID:           violation.CheckID,
				NodeID:            _node.ID,
				Name:              violation.Name,
				Attribute:         violation.Attribute,
				Message:           violation.Message,
				Severity:          violation.Severity,
				EvaluatedAt:       node.LastEvaluatedAt,
			})
		}

		return _node.UpsertViolations(ctx, violations)
	}

	return nil
}

func migrateNodes_CleanUp(ctx context.Context) error {
	nodes, _ := (&schema.Node{}).ListAll(ctx)
	for _, node := range nodes {
		err := node.Get(ctx)
		if err != nil {
			return err
		}

		if node.IsGateway {
			for clientID := range node.RelayedClients {
				client := &schema.Node{
					ID: clientID,
				}
				err = client.Get(ctx)
				if err != nil {
					// ignore if extclient or does not exist.
					if errors.Is(err, gorm.ErrRecordNotFound) {
						continue
					}

					return err
				}

				relayedByNodeID := node.ID
				client.RelayedByNodeID = &relayedByNodeID
				client.IsIGWClient = false

				_, ok := node.RelayedIGWClients[clientID]
				if ok {
					client.IsIGWClient = true
				}

				err = client.Upsert(ctx)
				if err != nil {
					return err
				}
			}
		}

		if node.RelayedByNodeID != nil {
			gateway := &schema.Node{
				ID: *node.RelayedByNodeID,
			}
			err = gateway.Get(ctx)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					node.RelayedByNodeID = nil
					node.IsIGWClient = false
				} else {
					return fmt.Errorf("failed to fetch gateway %s for node %s: %w", *node.RelayedByNodeID, node.ID, err)
				}
			} else {
				if !gateway.IsGateway {
					node.RelayedByNodeID = nil
					node.IsIGWClient = false
				} else if !gateway.IsInternetGateway {
					node.IsIGWClient = false
				}
			}

			err = node.Upsert(ctx)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
