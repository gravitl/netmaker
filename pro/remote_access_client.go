//go:build ee
// +build ee

package pro

import (
	"fmt"
	"time"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"golang.org/x/exp/slog"
)

const unauthorisedUserNodeCheckInterval = 3 * time.Minute

// AddUnauthorisedUserNodeHooks - adds hook to prevent access from unauthorised (expired) user nodes
func AddUnauthorisedUserNodeHooks() {
	slog.Debug("adding unauthorisedUserNode hook")
	logic.HookManagerCh <- models.HookDetails{
		Hook:     unauthorisedUserNodeHook,
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
	clients, err := logic.GetAllExtClients()
	if err != nil {
		slog.Error("error getting clients: ", "error", err)
		return err
	}

	currentTime := time.Now()
	validityDuration := logic.GetJwtValidityDuration()
	for _, user := range users {
		if user.PlatformRoleID == models.AdminRole ||
			user.PlatformRoleID == models.SuperAdminRole {
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
				if err := disableExtClient(&client); err != nil {
					slog.Error("error disabling user node", "error", err)
					continue // dont return but try for other clients
				}
			}
		}
	}

	slog.Debug("finished running unauthorisedUserNode hook")
	return nil
}

func disableExtClient(client *models.ExtClient) error {
	if newClient, err := logic.ToggleExtClientConnectivity(client, false); err != nil {
		return err
	} else {
		// publish peer update to ingress gateway
		if ingressNode, err := logic.GetNodeByID(newClient.IngressGatewayID); err == nil {
			if err = mq.PublishPeerUpdate(false); err != nil {
				slog.Error("error updating ext clients on", "ingress", ingressNode.ID.String(), "err", err.Error())
			}
			ingressHost, err := logic.GetHost(ingressNode.HostID.String())
			if err != nil {
				return err
			}
			nodes, err := logic.GetAllNodes()
			if err != nil {
				return err
			}
			go mq.PublishSingleHostPeerUpdate(ingressHost, nodes, nil, []models.ExtClient{*client}, false, nil)
		} else {
			return err
		}
	}

	return nil
}
