package mq

import (
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
)

func init() {
	logic.PublishHostRegistrationUpdates = publishHostRegistrationUpdates
	logic.RequestHostPullUpdate = requestHostPullUpdate
}

func publishHostRegistrationUpdates(host *schema.Host) error {
	if host == nil || !servercfg.IsMessageQueueBackend() {
		return nil
	}
	if err := HostUpdate(&models.HostUpdate{
		Action: models.RequestAck,
		Host:   *host,
	}); err != nil {
		return err
	}
	return PublishPeerUpdate(false)
}

func requestHostPullUpdate(host *schema.Host) error {
	if host == nil {
		return nil
	}
	return HostUpdate(&models.HostUpdate{
		Action: models.RequestPull,
		Host:   *host,
	})
}
