package migrate

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
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
	records, err := fetchAll(ctx, database.PENDING_USERS_TABLE_NAME)
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
	records, err := fetchAll(ctx, database.USER_INVITES_TABLE_NAME)
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

		logger.Log(4, fmt.Sprintf("migrating user invite %s", _userInvite.InviteCode))

		err = _userInvite.Create(ctx)
		if err != nil {
			logger.Log(4, fmt.Sprintf("migrating user invite (%s/%s) failed: %v", _userInvite.InviteCode, _userInvite.Email, err))
			return err
		}
	}

	return nil
}

func migrateNodes(ctx context.Context) error {
	records, err := fetchAll(ctx, database.NODES_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}

	for _, record := range records {
		var node models.Node
		err = json.Unmarshal([]byte(record), &node)
		if err != nil {
			return err
		}

		network := &schema.Network{
			Name: node.Network,
		}
		err = network.Get(ctx)
		if err != nil {
			return err
		}

		relayedClients := make(datatypes.JSONMap)
		for _, relayedNodeID := range node.RelayedNodes {
			relayedClients[relayedNodeID] = struct{}{}
		}

		relayedIGWClients := make(datatypes.JSONMap)
		for _, inetNodeClientID := range node.InetNodeReq.InetNodeClientIDs {
			relayedIGWClients[inetNodeClientID] = struct{}{}
		}

		tags := make(datatypes.JSONMap)
		for tagID := range node.Tags {
			tags[tagID.String()] = struct{}{}
		}

		_node := &schema.Node{
			ID:                                node.ID.String(),
			HostID:                            node.HostID.String(),
			NetworkID:                         network.ID,
			Address:                           node.Address.String(),
			Address6:                          node.Address6.String(),
			Connected:                         node.Connected,
			Action:                            node.Action,
			Status:                            node.Status,
			PendingDelete:                     node.PendingDelete,
			AutoAssignGateway:                 node.AutoAssignGateway,
			IsGateway:                         node.IsGw || node.IsRelay || node.IsIngressGateway,
			IsAutoRelay:                       node.IsAutoRelay,
			IsInternetGateway:                 node.IsGw && node.IsInternetGateway,
			RelayedClients:                    relayedClients,
			RelayedIGWClients:                 relayedIGWClients,
			RelayingNodeID:                    datatypes.NewNull(node.RelayedBy),
			IsIGWClient:                       node.IsRelayed && node.InternetGwID != "",
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

		logger.Log(4, fmt.Sprintf("migrating node %s", _node.ID))

		err = _node.Create(ctx)
		if err != nil {
			logger.Log(4, fmt.Sprintf("migrating node (%s) failed: %v", _node.ID, err))
			return err
		}

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

			err = _node.UpsertViolations(ctx, violations)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
