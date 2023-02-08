package queue

import (
	"encoding/json"
	"fmt"

	"github.com/gorilla/websocket"
	"github.com/gravitl/netmaker/logger"
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

// NodeUpdate -- publishes a node update
func NodeUpdate(node *models.Node) error {
	host, err := logic.GetHost(node.HostID.String())
	if err != nil {
		return err
	}
	event := models.Event{
		ID:    host.ID.String(),
		Topic: models.EventTopics.SendNodeUpdate,
	}
	event.Payload.Node = node
	logger.Log(1, "publishing node update to", host.Name, node.ID.String())
	return EventQueue.Enqueue(event)
}

// HostUpdate -- publishes a host update to clients
func HostUpdate(hostUpdate *models.HostUpdate) error {
	event := models.Event{
		ID:    hostUpdate.Host.ID.String(),
		Topic: models.EventTopics.SendHostUpdate,
	}
	event.Payload.HostUpdate = hostUpdate
	return EventQueue.Enqueue(event)
}

func sendNodeUpdate(e *models.Event) {
	data, err := json.Marshal(e)
	if err != nil {
		logger.Log(0, "failed to encode node update", err.Error())
	}
	if err = publish(data, e.ID); err != nil {
		logger.Log(0, "failed to send node update", err.Error())
	}
}

func sendHostUpdate(e *models.Event) {
	logger.Log(1, "publishing host update to "+e.ID)
	data, err := json.Marshal(e)
	if err != nil {
		logger.Log(0, "failed to encode host update", err.Error())
		return
	}
	if err = publish(data, e.ID); err != nil {
		logger.Log(0, "failed to send host update", err.Error())
	}
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
