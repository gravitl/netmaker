package controller

import (
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/servercfg"
)

func runServerPeerUpdate(network string, shouldPeerUpdate bool) error {
	err := logic.TimerCheckpoint()
	if err != nil {
		logger.Log(3, "error occurred on timer,", err.Error())
	}
	if servercfg.IsClientMode() != "on" {
		return nil
	}
	var currentServerNodeID, getErr = logic.GetNetworkServerNodeID(network)
	if err != nil {
		return getErr
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
