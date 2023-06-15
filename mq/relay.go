package mq

import (
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

func BroadCastRelayUpdate(relayReq models.RelayRequest) error {
	clients, err := logic.GetNetworkClients(relayReq.NetID)
	if err != nil {
		return err
	}
	// filter relay Node
	filteredClients := clients
	for i := len(filteredClients) - 1; i >= 0; i-- {
		if filteredClients[i].Node.ID.String() == relayReq.NodeID {
			filteredClients = append(filteredClients[:i], filteredClients[i+1:]...)
			break
		}
	}
	for _, relayedNodeID := range relayReq.RelayedNodes {
		relayedNode, err := logic.GetNodeByID(relayedNodeID)
		if err != nil {
			continue
		}

		h, err := logic.GetHost(relayedNode.HostID.String())
		if err != nil {
			continue
		}
		BroadcastDelPeer(h, filteredClients)
		FlushNetworkPeersToHost(&models.Client{Host: *h, Node: relayedNode}, clients)
	}
	relayNode, err := logic.GetNodeByID(relayReq.NodeID)
	if err != nil {
		return err
	}
	relayHost, err := logic.GetHost(relayNode.HostID.String())
	if err != nil {
		return err
	}

	return BroadcastAddOrUpdateNetworkPeer(&models.Client{Host: *relayHost, Node: relayNode}, true)
}

func BroadCastRelayRemoval(network string) error {
	clients, err := logic.GetNetworkClients(network)
	if err != nil {
		return err
	}
	for _, client := range clients {
		client := client
		go FlushNetworkPeersToHost(&client, clients)
	}
	return err
}
