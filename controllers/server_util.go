package controller

import (
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/serverctl"
)

func runServerPeerUpdate(network string, shouldPeerUpdate bool) error {
	if servercfg.Telemetry() == "on" {
		err := serverctl.TelemetryCheckpoint()
		if err != nil {
			logger.Log(1, "failed to send telemetry:", err.Error())
		}
	}
	if servercfg.IsClientMode() != "on" {
		return nil
	}
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
