package controller

import (
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/serverctl"
)

func runServerPeerUpdate() error {
	var serverData = models.ServerUpdateData{
		UpdatePeers: true,
	}
	serverctl.Push(serverData)
	var settings, err = serverctl.Pop()
	if err != nil {
		return err
	}
	return handlePeerUpdate(&settings.ServerNode)
}

func runServerUpdateIfNeeded(shouldPeersUpdate bool, serverNode *models.Node) error {
	// check if a peer/server update is needed
	var serverData = models.ServerUpdateData{
		UpdatePeers: shouldPeersUpdate,
	}
	if serverNode.IsServer == "yes" {
		serverData.ServerNode = *serverNode
	}
	serverctl.Push(serverData)

	return handleServerUpdate()
}

func handleServerUpdate() error {
	var settings, settingsErr = serverctl.Pop()
	if settingsErr != nil {
		return settingsErr
	}
	var currentServerNodeID, err = logic.GetNetworkServerNodeID(settings.ServerNode.Network)
	if err != nil {
		return err
	}
	// ensure server client is available
	if settings.UpdatePeers || (settings.ServerNode.ID == currentServerNodeID) {
		err = serverctl.SyncServerNetwork(&settings.ServerNode)
		if err != nil {
			logger.Log(1, "failed to sync,", settings.ServerNode.Network, ", error:", err.Error())
		}
	}
	// if peers should update, update peers on network
	if settings.UpdatePeers {
		var currentServerNode, err = logic.GetNodeByID(currentServerNodeID)
		if err != nil {
			return err
		}

		if err = handlePeerUpdate(&currentServerNode); err != nil {
			return err
		}
		logger.Log(1, "updated peers on network:", currentServerNode.Network)
	}
	// if the server node had an update, run the update function
	if settings.ServerNode.ID == currentServerNodeID {
		if err = logic.ServerUpdate(&settings.ServerNode); err != nil {
			return err
		}
		logger.Log(1, "server node:", settings.ServerNode.ID, "was updated")
	}
	return nil
}

// tells server to update it's peers
func handlePeerUpdate(serverNode *models.Node) error {
	logger.Log(1, "updating peers on network:", serverNode.Network)
	logic.SetNetworkServerPeers(serverNode)
	return nil
}
