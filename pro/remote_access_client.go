package pro

import (
	"fmt"
	"time"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slog"
)

const racAutoDisableCheckInterval = 3 * time.Minute

// AddRacHooks - adds hooks for Remote Access Client
func AddRacHooks() {
	slog.Debug("adding RAC autodisable hook")
	logic.HookManagerCh <- models.HookDetails{
		Hook:     racAutoDisableHook,
		Interval: racAutoDisableCheckInterval,
	}
}

// racAutoDisableHook - checks if RAC is enabled and if it is, checks if it should be disabled
func racAutoDisableHook() error {
	slog.Debug("running RAC autodisable hook")

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
	validityDuration := servercfg.GetJwtValidityDuration()
	for _, user := range users {
		if !currentTime.After(user.LastLoginTime.Add(validityDuration)) {
			continue
		}
		for _, client := range clients {
			if client.OwnerID == user.UserName {
				slog.Info(fmt.Sprintf("disabling ext client %s for user %s due to RAC autodisabling", client.ClientID, client.OwnerID))
				if err := disableExtClient(&client); err != nil {
					slog.Error("error disabling ext client in RAC autodisable hook", "error", err)
					return err
				}
			}
		}
	}

	slog.Debug("finished running RAC autodisable hook")
	return nil
}

func disableExtClient(client *models.ExtClient) error {
	update := models.CustomExtClient{
		Enabled:              false,
		ClientID:             client.ClientID,
		PublicKey:            client.PublicKey,
		DNS:                  client.DNS,
		ExtraAllowedIPs:      client.ExtraAllowedIPs,
		DeniedACLs:           client.DeniedACLs,
		RemoteAccessClientID: client.RemoteAccessClientID,
	}

	// update in DB
	newClient := logic.UpdateExtClient(client, &update)
	if err := logic.DeleteExtClient(client.Network, client.ClientID); err != nil {
		slog.Error("failed to delete ext client during update", "id", client.ClientID, "network", client.Network, "error", err)
		return err
	}
	if err := logic.SaveExtClient(&newClient); err != nil {
		slog.Error("failed to save updated ext client during update", "id", newClient.ClientID, "network", newClient.Network, "error", err)
		return err
	}

	// publish peer update to ingress gateway
	if ingressNode, err := logic.GetNodeByID(newClient.IngressGatewayID); err == nil {
		if err = mq.PublishPeerUpdate(); err != nil {
			slog.Error("error updating ext clients on", "ingress", ingressNode.ID.String(), "err", err.Error())
		}
	} else {
		return err
	}

	return nil
}
