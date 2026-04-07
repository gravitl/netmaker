package mq

import (
	"encoding/json"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slog"
)

type serverSyncMessage struct {
	Sender   string               `json:"sender"`
	SyncType logic.ServerSyncType `json:"sync_type"`
}

// InitServerSync wires up the logic.PublishServerSync hook so that
// mutations in the logic package can broadcast sync signals
// without importing mq (avoiding circular imports).
func InitServerSync() {
	logic.PublishServerSync = publishServerSync
}

func publishServerSync(syncType logic.ServerSyncType) {
	if mqclient == nil || !mqclient.IsConnectionOpen() {
		return
	}
	msg := serverSyncMessage{
		Sender:   servercfg.GetHostName(),
		SyncType: syncType,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("serversync: failed to marshal message", "error", err)
		return
	}
	topic := fmt.Sprintf("serversync/%s", servercfg.GetServer())
	token := mqclient.Publish(topic, 0, false, data)
	if !token.WaitTimeout(MQ_TIMEOUT * time.Second) {
		slog.Warn("serversync: publish timed out", "topic", topic)
	} else if token.Error() != nil {
		slog.Error("serversync: publish failed", "topic", topic, "error", token.Error())
	}
}

func handleServerSync(_ mqtt.Client, msg mqtt.Message) {
	var syncMsg serverSyncMessage
	if err := json.Unmarshal(msg.Payload(), &syncMsg); err != nil {
		slog.Error("serversync: failed to parse message", "error", err)
		return
	}
	if syncMsg.Sender == servercfg.GetHostName() {
		return
	}
	slog.Info("serversync: received sync", "from", syncMsg.Sender, "type", syncMsg.SyncType)

	switch syncMsg.SyncType {
	case logic.SyncTypeSettings:
		logic.InvalidateServerSettingsCache()
		logic.NotifyMetricExportIntervalChanged()
	case logic.SyncTypePeerUpdate:
		logic.InvalidateHostPeerCaches()
		go warmPeerCaches()
	case logic.SyncTypeIDPReset:
		if servercfg.IsMasterPod() {
			logic.ResetIDPSyncHook()
		}
	case logic.SyncTypeIDPSync:
		if servercfg.IsMasterPod() {
			logic.SyncFromIDP()
		}
	}
}
