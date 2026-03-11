package mq

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slog"
)

type serverSyncMessage struct {
	Sender string `json:"sender"`
	Action string `json:"action"`
	Key    string `json:"key,omitempty"`
}

// InitServerSync wires up the logic.OnCacheInvalidation hook so that
// mutations in the logic package can broadcast invalidation signals
// without importing mq (avoiding circular imports).
func InitServerSync() {
	logic.OnCacheInvalidation = publishServerSync
}

func publishServerSync(cacheType, key string) {
	if mqclient == nil || !mqclient.IsConnectionOpen() {
		return
	}
	msg := serverSyncMessage{
		Sender: servercfg.GetNodeID(),
		Action: "invalidate",
		Key:    key,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("serversync: failed to marshal message", "error", err)
		return
	}
	topic := fmt.Sprintf("serversync/%s/%s", servercfg.GetServer(), cacheType)
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
	if syncMsg.Sender == servercfg.GetNodeID() {
		return
	}
	parts := strings.Split(msg.Topic(), "/")
	if len(parts) < 3 {
		return
	}
	cacheType := parts[len(parts)-1]
	slog.Info("serversync: received invalidation", "from", syncMsg.Sender, "type", cacheType, "key", syncMsg.Key)

	switch cacheType {
	case "settings":
		logic.InvalidateServerSettingsCache()
	case "peerupdate":
		go func() {
			logic.InvalidateHostPeerCaches()
			warmPeerCaches()
		}()
	}
}
