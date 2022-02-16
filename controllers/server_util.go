package controller

import (
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

// updates local peers for a server on a given node's network
func runServerUpdate(node *models.Node, ifaceDelta bool) error {

	err := logic.TimerCheckpoint()
	if err != nil {
		logger.Log(3, "error occurred on timer,", err.Error())
	}

	if servercfg.IsClientMode() != "on" {
		return nil
	}
	var currentServerNode, getErr = logic.GetNetworkServerLeader(node.Network)
	if err != nil {
		return getErr
	}

	if err = logic.ServerUpdate(&currentServerNode, ifaceDelta); err != nil {
		logger.Log(1, "server node:", currentServerNode.ID, "failed update")
		return err
	}
	return nil
}
