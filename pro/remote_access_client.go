//go:build ee
// +build ee

package pro

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"golang.org/x/exp/slog"
)

const unauthorisedUserNodeCheckInterval = 3 * time.Minute

// AddUnauthorisedUserNodeHooks - adds hook to prevent access from unauthorised (expired) user nodes
func AddUnauthorisedUserNodeHooks() {
	slog.Debug("adding unauthorisedUserNode hook")
	logic.HookManagerCh <- models.HookDetails{
		ID:       "unauthorised-users-hook",
		Hook:     logic.WrapHook(unauthorisedUserNodeHook),
		Interval: unauthorisedUserNodeCheckInterval,
	}
}

// unauthorisedUserNodeHook - checks if a user node should be disabled, using the user's last login time
func unauthorisedUserNodeHook() error {
	slog.Debug("running unauthorisedUserNode hook")

	users, err := logic.GetUsers()
	if err != nil {
		slog.Error("error getting users: ", "error", err)
		return err
	}
	ctx := logic.DefaultScope(db.WithContext(context.TODO()))
	clients, err := logic.GetAllExtClients(ctx)
	if err != nil {
		slog.Error("error getting clients: ", "error", err)
		return err
	}

	currentTime := time.Now()
	validityDuration := logic.GetJwtValidityDuration(ctx)
	for _, user := range users {
		if user.PlatformRoleID == schema.AdminRole ||
			user.PlatformRoleID == schema.SuperAdminRole {
			continue
		}
		if !currentTime.After(user.LastLoginTime.Add(validityDuration)) {
			continue
		}
		for _, client := range clients {
			if client.RemoteAccessClientID == "" {
				continue
			}
			if (client.OwnerID == user.UserName) &&
				client.Enabled {
				slog.Info(fmt.Sprintf("disabling user node %s for user %s: auth token expired", client.ClientID, client.OwnerID))
				if err := disableExtClient(ctx, &client); err != nil {
					slog.Error("error disabling user node", "error", err)
					continue // dont return but try for other clients
				}
			}
		}
	}

	slog.Debug("finished running unauthorisedUserNode hook")
	return nil
}

func disableExtClient(ctx context.Context, client *models.ExtClient) error {
	if newClient, err := logic.ToggleExtClientConnectivity(ctx, client, false); err != nil {
		return err
	} else {
		// publish peer update to ingress gateway
		if ingressNode, err := logic.GetNodeByID(newClient.IngressGatewayID); err == nil {
			ingressHost := &schema.Host{
				ID: ingressNode.HostID,
			}
			err := ingressHost.Get(ctx)
			if err != nil {
				return err
			}
			ctx := scope.WithContext(db.WithContext(context.Background()), scope.TenantScope, ingressHost.TenantID)
			if err = mq.PublishPeerUpdate(ctx, false); err != nil {
				slog.Error("error updating ext clients on", "ingress", ingressNode.ID.String(), "err", err.Error())
			}
			nodes, err := logic.GetAllNodes()
			if err != nil {
				return err
			}
			go mq.PublishSingleHostPeerUpdate(ctx, ingressHost, nodes, nil, nil, []models.ExtClient{*client}, false, nil)
		} else {
			return err
		}
	}

	return nil
}
