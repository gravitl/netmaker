package queue

import (
	"encoding/json"
	"fmt"

	"github.com/gorilla/websocket"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

// PublishAllPeerUpdate - publishes a peer update to
// all hosts with current connections
func PublishAllPeerUpdate() {
	const publishAllID = "pub-all"
	event := models.Event{
		ID:    publishAllID,
		Topic: models.EventTopics.SendAllHostPeerUpdate,
	}
	EventQueue.Enqueue(event)
}

func publishPeerUpdates(e *models.Event) {
	hostMap, err := logic.GetHostsMap()
	if err != nil {
		return
	}

	ConnMap.Range(func(k, v interface{}) bool {
		host, ok := hostMap[k.(string)] // in future can also handle http responses
		if ok {                         // ensure ID is a legitimate host
			conn := v.(*websocket.Conn)
			if conn == nil {
				return false
			}
			_ = publishHostPeerUpdate(host)
		}
		return true
	})
}

func publishHostPeerUpdate(host *models.Host) error {

	peerUpdate, err := logic.GetPeerUpdateForHost(host)
	if err != nil {
		return err
	}
	if host.ProxyEnabled {
		proxyUpdate, err := logic.GetProxyUpdateForHost(host)
		if err != nil {
			return err
		}
		proxyUpdate.Action = models.ProxyUpdate
		peerUpdate.ProxyUpdate = proxyUpdate
	}

	event := models.Event{
		ID:    host.ID.String(),
		Topic: models.EventTopics.SendHostPeerUpdate,
	}
	event.Payload.HostPeerUpdate = &peerUpdate

	data, err := json.Marshal(&event)
	if err != nil {
		return err
	}
	return publish(data, host.ID.String())
}

func publish(data []byte, hostID string) error {
	val, ok := ConnMap.Load(hostID)
	if ok {
		conn := val.(*websocket.Conn)
		if conn != nil {
			return conn.WriteMessage(websocket.TextMessage, data)
		}
	}
	return fmt.Errorf("message send failure for host connection %s", hostID)
}
