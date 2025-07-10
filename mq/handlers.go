package mq

import (
	"encoding/json"
	"os"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/hostactions"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slog"
)

// UpdateMetrics  message Handler -- handles updates from client nodes for metrics
var UpdateMetrics = func(client mqtt.Client, msg mqtt.Message) {
}

var UpdateMetricsFallBack = func(nodeid string, newMetrics models.Metrics) {}

// DefaultHandler default message queue handler  -- NOT USED
func DefaultHandler(client mqtt.Client, msg mqtt.Message) {
	slog.Info("mqtt default handler", "topic", msg.Topic(), "message", msg.Payload())
}

// UpdateNode  message Handler -- handles updates from client nodes
func UpdateNode(client mqtt.Client, msg mqtt.Message) {
	id, err := GetID(msg.Topic())
	if err != nil {
		slog.Error("error getting node.ID ", "topic", msg.Topic(), "error", err)
		return
	}
	currentNode, err := logic.GetNodeByID(id)
	if err != nil {
		slog.Error("error getting node", "id", id, "error", err)
		return
	}
	decrypted, decryptErr := DecryptMsg(&currentNode, msg.Payload())
	if decryptErr != nil {
		slog.Error("failed to decrypt message for node", "id", id, "error", decryptErr)
		return
	}
	var newNode models.Node
	if err := json.Unmarshal(decrypted, &newNode); err != nil {
		slog.Error("error unmarshaling payload", "error", err)
		return
	}

	ifaceDelta := logic.IfaceDelta(&currentNode, &newNode)
	newNode.SetLastCheckIn()
	if err := logic.UpdateNode(&currentNode, &newNode); err != nil {
		slog.Error("error saving node", "id", id, "error", err)
		return
	}
	if ifaceDelta { // reduce number of unneeded updates, by only sending on iface changes
		if !newNode.Connected {
			err = PublishDeletedNodePeerUpdate(&newNode)
			host, err := logic.GetHost(newNode.HostID.String())
			if err != nil {
				slog.Error("failed to get host for the node", "nodeid", newNode.ID.String(), "error", err)
				return
			}
			allNodes, err := logic.GetAllNodes()
			if err == nil {
				PublishSingleHostPeerUpdate(host, allNodes, nil, nil, false, nil)
			}
		} else {
			err = PublishPeerUpdate(false)
		}
		if err != nil {
			slog.Warn("error updating peers when node informed the server of an interface change", "nodeid", currentNode.ID, "error", err)
		}
	}

	slog.Info("updated node", "id", id, "newnodeid", newNode.ID)
}

// UpdateHost  message Handler -- handles host updates from clients
func UpdateHost(client mqtt.Client, msg mqtt.Message) {
	id, err := GetID(msg.Topic())
	if err != nil {
		slog.Error("error getting host.ID sent on ", "topic", msg.Topic(), "error", err)
		return
	}
	currentHost, err := logic.GetHost(id)
	if err != nil {
		slog.Error("error getting host", "id", id, "error", err)
		return
	}
	decrypted, decryptErr := decryptMsgWithHost(currentHost, msg.Payload())
	if decryptErr != nil {
		slog.Error("failed to decrypt message for host", "id", id, "name", currentHost.Name, "error", decryptErr)
		return
	}
	var hostUpdate models.HostUpdate
	if err := json.Unmarshal(decrypted, &hostUpdate); err != nil {
		slog.Error("error unmarshaling payload", "error", err)
		return
	}
	slog.Info("recieved host update", "name", hostUpdate.Host.Name, "id", hostUpdate.Host.ID)
	var sendPeerUpdate bool
	var replacePeers bool
	switch hostUpdate.Action {
	case models.CheckIn:
		sendPeerUpdate = HandleHostCheckin(&hostUpdate.Host, currentHost)
	case models.Acknowledgement:
		hu := hostactions.GetAction(currentHost.ID.String())
		if hu != nil {
			if err = HostUpdate(hu); err != nil {
				slog.Error("failed to send new node to host", "name", hostUpdate.Host.Name, "id", currentHost.ID, "error", err)
				return
			} else {
				nodes, err := logic.GetAllNodes()
				if err != nil {
					return
				}
				if err = PublishSingleHostPeerUpdate(currentHost, nodes, nil, nil, false, nil); err != nil {
					slog.Error("failed peers publish after join acknowledged", "name", hostUpdate.Host.Name, "id", currentHost.ID, "error", err)
					return
				}
			}
		}
	case models.UpdateHost:
		if hostUpdate.Host.PublicKey != currentHost.PublicKey {
			//remove old peer entry
			replacePeers = true
		}
		sendPeerUpdate = logic.UpdateHostFromClient(&hostUpdate.Host, currentHost)
		err := logic.UpsertHost(currentHost)
		if err != nil {
			slog.Error("failed to update host", "id", currentHost.ID, "error", err)
			return
		}
	case models.DeleteHost:
		if servercfg.GetBrokerType() == servercfg.EmqxBrokerType {
			// delete EMQX credentials for host
			if err := emqx.DeleteEmqxUser(currentHost.ID.String()); err != nil {
				slog.Error("failed to remove host credentials from EMQX", "id", currentHost.ID, "error", err)
			}
		}

		// notify of deleted peer change
		go func(host models.Host) {
			for _, nodeID := range host.Nodes {
				node, err := logic.GetNodeByID(nodeID)
				if err == nil {
					var gwClients []models.ExtClient
					if node.IsIngressGateway {
						gwClients = logic.GetGwExtclients(node.ID.String(), node.Network)
					}
					go PublishMqUpdatesForDeletedNode(node, false, gwClients)
				}

			}
		}(*currentHost)

		if err := logic.DisassociateAllNodesFromHost(currentHost.ID.String()); err != nil {
			slog.Error("failed to delete all nodes of host", "id", currentHost.ID, "error", err)
			return
		}
		if err := logic.RemoveHostByID(currentHost.ID.String()); err != nil {
			slog.Error("failed to delete host", "id", currentHost.ID, "error", err)
			return
		}
		if servercfg.IsDNSMode() {
			logic.SetDNS()
		}
		sendPeerUpdate = true
	case models.SignalHost:
		signalPeer(hostUpdate.Signal)

	}

	if sendPeerUpdate {
		err := PublishPeerUpdate(replacePeers)
		if err != nil {
			slog.Error("failed to publish peer update", "error", err)
		}
	}
}

func signalPeer(signal models.Signal) {

	if signal.ToHostPubKey == "" {
		msg := "insufficient data to signal peer"
		logger.Log(0, msg)
		return
	}
	signal.IsPro = servercfg.IsPro
	peerHost, err := logic.GetHost(signal.ToHostID)
	if err != nil {
		slog.Error("failed to signal, peer host not found", "error", err)
		return
	}

	err = HostUpdate(&models.HostUpdate{
		Action: models.SignalHost,
		Host:   *peerHost,
		Signal: signal,
	})
	if err != nil {
		slog.Error("failed to publish signal to peer", "error", err)
	}
}

// ClientPeerUpdate  message handler -- handles updating peers after signal from client nodes
func ClientPeerUpdate(client mqtt.Client, msg mqtt.Message) {
	id, err := GetID(msg.Topic())
	if err != nil {
		slog.Error("error getting node.ID sent on ", "topic", msg.Topic(), "error", err)
		return
	}
	currentNode, err := logic.GetNodeByID(id)
	if err != nil {
		slog.Error("error getting node", "id", id, "error", err)
		return
	}
	decrypted, decryptErr := DecryptMsg(&currentNode, msg.Payload())
	if decryptErr != nil {
		slog.Error("failed to decrypt message for node", "id", id, "error", decryptErr)
		return
	}
	switch decrypted[0] {
	case ncutils.ACK:
		// do we still need this
	case ncutils.DONE:
		if err = PublishPeerUpdate(false); err != nil {
			slog.Error("error publishing peer update for node", "id", currentNode.ID, "error", err)
			return
		}
	}

	slog.Info("sent peer updates after signal received from", "id", id)
}

func HandleHostCheckin(h, currentHost *models.Host) bool {
	if h == nil {
		return false
	}

	for i := range currentHost.Nodes {
		currNodeID := currentHost.Nodes[i]
		node, err := logic.GetNodeByID(currNodeID)
		if err != nil {
			if database.IsEmptyRecord(err) {
				fakeNode := models.Node{}
				fakeNode.ID, _ = uuid.Parse(currNodeID)
				fakeNode.Action = models.NODE_DELETE
				fakeNode.PendingDelete = true
				if err := NodeUpdate(&fakeNode); err != nil {
					slog.Warn("failed to inform host to remove node", "host", currentHost.Name, "hostid", currentHost.ID, "nodeid", currNodeID, "error", err)
				}
			}
			continue
		}
		if err := logic.UpdateNodeCheckin(&node); err != nil {
			slog.Warn("failed to update node on checkin", "nodeid", node.ID, "error", err)
		}
	}

	for i := range h.Interfaces {
		h.Interfaces[i].AddressString = h.Interfaces[i].Address.String()
	}
	/// version or firewall in use change does not require a peerUpdate
	if h.Version != currentHost.Version || h.FirewallInUse != currentHost.FirewallInUse {
		currentHost.FirewallInUse = h.FirewallInUse
		currentHost.Version = h.Version
		if err := logic.UpsertHost(currentHost); err != nil {
			slog.Error("failed to update host after check-in", "name", h.Name, "id", h.ID, "error", err)
			return false
		}
	}
	ifaceDelta := len(h.Interfaces) != len(currentHost.Interfaces) ||
		!h.EndpointIP.Equal(currentHost.EndpointIP) ||
		(len(h.NatType) > 0 && h.NatType != currentHost.NatType) ||
		h.DefaultInterface != currentHost.DefaultInterface ||
		(h.ListenPort != 0 && h.ListenPort != currentHost.ListenPort) ||
		(h.WgPublicListenPort != 0 && h.WgPublicListenPort != currentHost.WgPublicListenPort) || (!h.EndpointIPv6.Equal(currentHost.EndpointIPv6))
	if ifaceDelta { // only save if something changes
		if !h.EndpointIP.Equal(currentHost.EndpointIP) || !h.EndpointIPv6.Equal(currentHost.EndpointIPv6) || currentHost.Location == "" {
			if h.EndpointIP != nil {
				h.Location = logic.GetHostLocInfo(h.EndpointIP.String(), os.Getenv("IP_INFO_TOKEN"))
			} else if h.EndpointIPv6 != nil {
				h.Location = logic.GetHostLocInfo(h.EndpointIPv6.String(), os.Getenv("IP_INFO_TOKEN"))
			}
		}
		currentHost.EndpointIP = h.EndpointIP
		currentHost.EndpointIPv6 = h.EndpointIPv6
		currentHost.Interfaces = h.Interfaces
		currentHost.DefaultInterface = h.DefaultInterface
		currentHost.NatType = h.NatType
		if h.Location != "" {
			currentHost.Location = h.Location
		}
		if h.ListenPort != 0 {
			currentHost.ListenPort = h.ListenPort
		}
		if h.WgPublicListenPort != 0 {
			currentHost.WgPublicListenPort = h.WgPublicListenPort
		}
		if err := logic.UpsertHost(currentHost); err != nil {
			slog.Error("failed to update host after check-in", "name", h.Name, "id", h.ID, "error", err)
			return false
		}
		slog.Info("updated host after check-in", "name", currentHost.Name, "id", currentHost.ID)
	}

	slog.Info("check-in processed for host", "name", h.Name, "id", h.ID)
	return ifaceDelta
}
