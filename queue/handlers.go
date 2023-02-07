package queue

import (
	"fmt"

	"github.com/gorilla/websocket"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
)

// holds a map of funcs
// based on topic to handle an event
var handlerFuncs map[int]func(*models.Event)

// initializes the map of functions
func initializeHandlers() {
	handlerFuncs = make(map[int]func(*models.Event))
	handlerFuncs[models.EventTopics.NodeUpdate] = nodeUpdate
	handlerFuncs[models.EventTopics.Test] = test
	handlerFuncs[models.EventTopics.HostUpdate] = hostUpdate
	handlerFuncs[models.EventTopics.Ping] = ping
	handlerFuncs[models.EventTopics.Metrics] = updateMetrics
	handlerFuncs[models.EventTopics.ClientUpdate] = clientPeerUpdate
}

func test(e *models.Event) {
	val, ok := ConnMap.Load(e.ID)
	if ok {
		conn := val.(*websocket.Conn)
		if conn != nil {
			conn.WriteMessage(websocket.TextMessage, []byte("success"))
		}
	}
}

func ping(e *models.Event) {
	node, err := logic.GetNodeByID(e.ID)
	if err != nil {
		logger.Log(3, "mq-ping error getting node: ", err.Error())
		record, err := database.FetchRecord(database.NODES_TABLE_NAME, e.ID)
		if err != nil {
			logger.Log(3, "error reading database ", err.Error())
			return
		}
		logger.Log(3, "record from database")
		logger.Log(3, record)
		return
	}

	checkin := e.Payload.NodeCheckin
	if checkin == nil {
		logger.Log(0, "failed to complete checkin for node", node.ID.String())
	}
	host, err := logic.GetHost(node.HostID.String())
	if err != nil {
		logger.Log(0, "error retrieving host for node ", node.ID.String(), err.Error())
		return
	}
	node.SetLastCheckIn()
	host.Version = checkin.Version
	node.Connected = checkin.Connected
	host.Interfaces = checkin.Ifaces
	for i := range host.Interfaces {
		host.Interfaces[i].AddressString = host.Interfaces[i].Address.String()
	}
	if err := logic.UpdateNode(&node, &node); err != nil {
		logger.Log(0, "error updating node", node.ID.String(), " on checkin", err.Error())
		return
	}

	logger.Log(3, "ping processed for node", node.ID.String())
}

func nodeUpdate(e *models.Event) {
	currentNode, err := logic.GetNodeByID(e.ID)
	if err != nil {
		logger.Log(1, "error getting node ", e.ID, err.Error())
		return
	}

	newNode := e.Payload.Node
	if newNode == nil {
		logger.Log(0, "failed to update node", currentNode.ID.String())
	}

	ifaceDelta := logic.IfaceDelta(&currentNode, newNode)
	if servercfg.Is_EE && ifaceDelta {
		if err = logic.EnterpriseResetAllPeersFailovers(currentNode.ID, currentNode.Network); err != nil {
			logger.Log(1, "failed to reset failover list during node update", currentNode.ID.String(), currentNode.Network)
		}
	}
	newNode.SetLastCheckIn()
	if err := logic.UpdateNode(&currentNode, newNode); err != nil {
		logger.Log(1, "error saving node", err.Error())
		return
	}
	if ifaceDelta { // reduce number of unneeded updates, by only sending on iface changes
		// TODO handle publishing udpates
		// if err = PublishPeerUpdate(); err != nil {
		// 	logger.Log(0, "error updating peers when node", currentNode.ID.String(), "informed the server of an interface change", err.Error())
		// }
	}

	logger.Log(1, "updated node", newNode.ID.String())
}

func hostUpdate(e *models.Event) {

	currentHost, err := logic.GetHost(e.ID)
	if err != nil {
		logger.Log(1, "error getting host ", e.ID, err.Error())
		return
	}

	hostUpdate := e.Payload.HostUpdate
	if hostUpdate == nil {
		logger.Log(0, "failed to update host", currentHost.Name, currentHost.ID.String())
	}
	logger.Log(3, fmt.Sprintf("recieved host update: %s\n", hostUpdate.Host.ID.String()))
	var sendPeerUpdate bool
	switch hostUpdate.Action {
	case models.UpdateHost:
		sendPeerUpdate = logic.UpdateHostFromClient(&hostUpdate.Host, currentHost)
		err := logic.UpsertHost(currentHost)
		if err != nil {
			logger.Log(0, "failed to update host: ", currentHost.ID.String(), err.Error())
			return
		}
	case models.DeleteHost:
		if err := logic.DisassociateAllNodesFromHost(currentHost.ID.String()); err != nil {
			logger.Log(0, "failed to delete all nodes of host: ", currentHost.ID.String(), err.Error())
			return
		}
		if err := logic.RemoveHostByID(currentHost.ID.String()); err != nil {
			logger.Log(0, "failed to delete host: ", currentHost.ID.String(), err.Error())
			return
		}
		sendPeerUpdate = true
	}
	// TODO handle publishing a peer update
	if sendPeerUpdate {
		// 	err := PublishPeerUpdate()
		// 	if err != nil {
		// 		logger.Log(0, "failed to pulish peer update: ", err.Error())
		// 	}
	}
}

func updateMetrics(e *models.Event) {
	if servercfg.Is_EE {
		id := e.ID
		currentNode, err := logic.GetNodeByID(id)
		if err != nil {
			logger.Log(1, "error getting node ", id, err.Error())
			return
		}

		var newMetrics = e.Payload.Metrics
		if newMetrics == nil {
			logger.Log(1, "provided metrics were nil for node", id)
			return
		}
		shouldUpdate := updateNodeMetrics(&currentNode, newMetrics)

		if err = logic.UpdateMetrics(id, newMetrics); err != nil {
			logger.Log(1, "faield to update node metrics", id, err.Error())
			return
		}
		// TODO adapt metrics exporter..
		// if servercfg.IsMetricsExporter() {
		// 	if err := pushMetricsToExporter(newMetrics); err != nil {
		// 		logger.Log(2, fmt.Sprintf("failed to push node: [%s] metrics to exporter, err: %v",
		// 			currentNode.ID, err))
		// 	}
		// }

		if newMetrics.Connectivity != nil {
			err := logic.EnterpriseFailoverFunc(&currentNode)
			if err != nil {
				logger.Log(0, "failed to failover for node", currentNode.ID.String(), "on network", currentNode.Network, "-", err.Error())
			}
		}

		if shouldUpdate {
			logger.Log(2, "updating peers after node", currentNode.ID.String(), currentNode.Network, "detected connectivity issues")
			// host, err := logic.GetHost(currentNode.HostID.String())
			// if err == nil {
			// 	if err = PublishSingleHostUpdate(host); err != nil {
			// 		logger.Log(0, "failed to publish update after failover peer change for node", currentNode.ID.String(), currentNode.Network)
			// 	}
			// }
			// TODO publish a single host update
		}

		logger.Log(1, "updated node metrics", id)
	}
}

func clientPeerUpdate(e *models.Event) {
	id := e.ID
	_, err := logic.GetNodeByID(id)
	if err != nil {
		logger.Log(1, "error getting node ", id, err.Error())
		return
	}
	action := e.Payload.Action
	switch action {
	case ncutils.ACK:
		//do we still need this
	case ncutils.DONE:
		// TODO publish a peer update to the calling node
	}

	logger.Log(1, "sent peer updates after signal received from", id)
}
