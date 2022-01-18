package controller

import (
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
)

func runServerPeerUpdate(network string, shouldPeerUpdate bool) error {

	var currentServerNodeID, err = logic.GetNetworkServerNodeID(network)
	if err != nil {
		return err
	}
	var currentServerNode, currErr = logic.GetNodeByID(currentServerNodeID)
	if currErr != nil {
		return currErr
	}
	if err = logic.ServerUpdate(&currentServerNode, shouldPeerUpdate); err != nil {
		logger.Log(1, "server node:", currentServerNode.ID, "failed update")
		return err
	}
	return nil
}

// func runServerUpdateIfNeeded(shouldPeersUpdate bool, node models.Node) error {
// 	// check if a peer/server update is needed
// 	var serverData = models.ServerUpdateData{
// 		UpdatePeers: shouldPeersUpdate,
// 		Node:        node,
// 	}
// 	serverctl.Push(serverData)

// 	return handleServerUpdate()
// }

// func handleServerUpdate() error {
// 	var settings, settingsErr = serverctl.Pop()
// 	if settingsErr != nil {
// 		return settingsErr
// 	}
// 	var currentServerNodeID, err = logic.GetNetworkServerNodeID(settings.Node.Network)
// 	if err != nil {
// 		return err
// 	}
// 	// ensure server client is available
// 	if settings.UpdatePeers || (settings.Node.ID == currentServerNodeID) {
// 		err = serverctl.SyncServerNetwork(&settings.Node)
// 		if err != nil {
// 			logger.Log(1, "failed to sync,", settings.Node.Network, ", error:", err.Error())
// 		}
// 	}
// 	// if peers should update, update peers on network
// 	if settings.UpdatePeers {
// 		if err = handlePeerUpdate(&settings.Node); err != nil {
// 			return err
// 		}
// 		logger.Log(1, "updated peers on network:", settings.Node.Network)
// 	}
// 	// if the server node had an update, run the update function
// 	if settings.Node.ID == currentServerNodeID {
// 		if err = logic.ServerUpdate(&settings.Node); err != nil {
// 			return err
// 		}
// 		logger.Log(1, "server node:", settings.Node.ID, "was updated")
// 	}
// 	return nil
// }

// // tells server to update it's peers
// func handlePeerUpdate(node *models.Node) error {
// 	logger.Log(1, "updating peers on network:", node.Network)
// 	var currentServerNodeID, err = logic.GetNetworkServerNodeID(node.Network)
// 	if err != nil {
// 		return err
// 	}
// 	var currentServerNode, currErr = logic.GetNodeByID(currentServerNodeID)
// 	if currErr != nil {
// 		return currErr
// 	}
// 	if err = logic.ServerUpdate(&currentServerNode); err != nil {
// 		logger.Log(1, "server node:", currentServerNode.ID, "failed update")
// 		return err
// 	}
// 	logger.Log(1, "finished a peer update for network,", currentServerNode.Network)
// 	return nil
// }
