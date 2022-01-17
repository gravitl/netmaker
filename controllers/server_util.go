package controller

import (
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/serverctl"
)

func runServerUpdateIfNeeded(currentNode *models.Node, newNode *models.Node) error {
	// check if a peer/server update is needed
	var serverData = serverctl.ServerUpdateData{
		UpdatePeers: logic.ShouldPeersUpdate(currentNode, newNode),
	}
	if currentNode.IsServer == "yes" {
		serverData.ServerNode = *currentNode
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
		serverctl.SyncServerNetwork(&settings.ServerNode)
	}
	// if peers should update, update peers on network
	if settings.UpdatePeers {
		if err = handlePeerUpdate(&settings.ServerNode); err != nil {
			return err
		}
		logger.Log(1, "updated peers on network:", settings.ServerNode.Network)
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
