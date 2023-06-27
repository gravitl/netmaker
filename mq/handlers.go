package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/hostactions"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slog"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// DefaultHandler default message queue handler  -- NOT USED
func DefaultHandler(client mqtt.Client, msg mqtt.Message) {
	slog.Info("mqtt default handler", "topic", msg.Topic(), "message", msg.Payload())
}

// UpdateNode  message Handler -- handles updates from client nodes
func UpdateNode(client mqtt.Client, msg mqtt.Message) {
	id, err := getID(msg.Topic())
	if err != nil {
		slog.Error("error getting node.ID ", "topic", msg.Topic(), "error", err)
		return
	}
	currentNode, err := logic.GetNodeByID(id)
	if err != nil {
		slog.Error("error getting node", "id", id, "error", err)
		return
	}
	decrypted, decryptErr := decryptMsg(&currentNode, msg.Payload())
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
	if servercfg.Is_EE && ifaceDelta {
		if err = logic.EnterpriseResetAllPeersFailovers(currentNode.ID, currentNode.Network); err != nil {
			slog.Warn("failed to reset failover list during node update", "nodeid", currentNode.ID, "network", currentNode.Network)
		}
	}
	newNode.SetLastCheckIn()
	if err := logic.UpdateNode(&currentNode, &newNode); err != nil {
		slog.Error("error saving node", "id", id, "error", err)
		return
	}
	if ifaceDelta { // reduce number of unneeded updates, by only sending on iface changes
		if err = PublishPeerUpdate(); err != nil {
			slog.Warn("error updating peers when node informed the server of an interface change", "nodeid", currentNode.ID, "error", err)
		}
	}

	slog.Info("updated node", "id", id, "newnodeid", newNode.ID)
}

// UpdateHost  message Handler -- handles host updates from clients
func UpdateHost(client mqtt.Client, msg mqtt.Message) {
	id, err := getID(msg.Topic())
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
		slog.Error("failed to decrypt message for host", "id", id, "error", decryptErr)
		return
	}
	var hostUpdate models.HostUpdate
	if err := json.Unmarshal(decrypted, &hostUpdate); err != nil {
		slog.Error("error unmarshaling payload", "error", err)
		return
	}
	slog.Info("recieved host update", "name", hostUpdate.Host.Name, "id", hostUpdate.Host.ID)
	var sendPeerUpdate bool
	switch hostUpdate.Action {
	case models.CheckIn:
		sendPeerUpdate = handleHostCheckin(&hostUpdate.Host, currentHost)
	case models.Acknowledgement:
		hu := hostactions.GetAction(currentHost.ID.String())
		if hu != nil {
			if err = HostUpdate(hu); err != nil {
				slog.Error("failed to send new node to host", "name", hostUpdate.Host.Name, "id", currentHost.ID, "error", err)
				return
			} else {
				if servercfg.GetBrokerType() == servercfg.EmqxBrokerType {
					if err = AppendNodeUpdateACL(hu.Host.ID.String(), hu.Node.Network, hu.Node.ID.String(), servercfg.GetServer()); err != nil {
						slog.Error("failed to add ACLs for EMQX node", "error", err)
						return
					}
				}
				if err = PublishSingleHostPeerUpdate(context.Background(), currentHost, nil, nil); err != nil {
					slog.Error("failed peers publish after join acknowledged", "name", hostUpdate.Host.Name, "id", currentHost.ID, "error", err)
					return
				}
				if err = handleNewNodeDNS(&hu.Host, &hu.Node); err != nil {
					slog.Error("failed to send dns update after node added to host", "name", hostUpdate.Host.Name, "id", currentHost.ID, "error", err)
					return
				}
			}
		}
	case models.UpdateHost:
		if hostUpdate.Host.PublicKey != currentHost.PublicKey {
			//remove old peer entry
			peerUpdate := models.HostPeerUpdate{
				ServerVersion: servercfg.GetVersion(),
				Peers: []wgtypes.PeerConfig{
					{
						PublicKey: currentHost.PublicKey,
						Remove:    true,
					},
				},
			}
			data, err := json.Marshal(&peerUpdate)
			if err != nil {
				slog.Error("failed to marshal peer update", "error", err)
			}
			hosts := logic.GetRelatedHosts(hostUpdate.Host.ID.String())
			server := servercfg.GetServer()
			for _, host := range hosts {
				publish(&host, fmt.Sprintf("peers/host/%s/%s", host.ID.String(), server), data)
			}

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
			if err := DeleteEmqxUser(currentHost.ID.String()); err != nil {
				slog.Error("failed to remove host credentials from EMQX", "id", currentHost.ID, "error", err)
				return
			}
		}
		if err := logic.DisassociateAllNodesFromHost(currentHost.ID.String()); err != nil {
			slog.Error("failed to delete all nodes of host", "id", currentHost.ID, "error", err)
			return
		}
		if err := logic.RemoveHostByID(currentHost.ID.String()); err != nil {
			slog.Error("failed to delete host", "id", currentHost.ID, "error", err)
			return
		}
		sendPeerUpdate = true
	case models.RegisterWithTurn:
		if servercfg.IsUsingTurn() {
			err = logic.RegisterHostWithTurn(hostUpdate.Host.ID.String(), hostUpdate.Host.HostPass)
			if err != nil {
				slog.Error("failed to register host with turn server", "id", currentHost.ID, "error", err)
				return
			}
		}

	}

	if sendPeerUpdate {
		err := PublishPeerUpdate()
		if err != nil {
			slog.Error("failed to publish peer update", "error", err)
		}
	}
	// if servercfg.Is_EE && ifaceDelta {
	// 	if err = logic.EnterpriseResetAllPeersFailovers(currentHost.ID.String(), currentHost.Network); err != nil {
	// 		logger.Log(1, "failed to reset failover list during node update", currentHost.ID.String(), currentHost.Network)
	// 	}
	// }
}

// UpdateMetrics  message Handler -- handles updates from client nodes for metrics
func UpdateMetrics(client mqtt.Client, msg mqtt.Message) {
	if servercfg.Is_EE {
		id, err := getID(msg.Topic())
		if err != nil {
			slog.Error("error getting ID sent on ", "topic", msg.Topic(), "error", err)
			return
		}
		currentNode, err := logic.GetNodeByID(id)
		if err != nil {
			slog.Error("error getting node", "id", id, "error", err)
			return
		}
		decrypted, decryptErr := decryptMsg(&currentNode, msg.Payload())
		if decryptErr != nil {
			slog.Error("failed to decrypt message for node", "id", id, "error", decryptErr)
			return
		}

		var newMetrics models.Metrics
		if err := json.Unmarshal(decrypted, &newMetrics); err != nil {
			slog.Error("error unmarshaling payload", "error", err)
			return
		}

		shouldUpdate := updateNodeMetrics(&currentNode, &newMetrics)

		if err = logic.UpdateMetrics(id, &newMetrics); err != nil {
			slog.Error("failed to update node metrics", "id", id, "error", err)
			return
		}
		if servercfg.IsMetricsExporter() {
			if err := pushMetricsToExporter(newMetrics); err != nil {
				slog.Error("failed to push node metrics to exporter", "id", currentNode.ID, "error", err)
			}
		}

		if newMetrics.Connectivity != nil {
			err := logic.EnterpriseFailoverFunc(&currentNode)
			if err != nil {
				slog.Error("failed to failover for node", "id", currentNode.ID, "network", currentNode.Network, "error", err)
			}
		}

		if shouldUpdate {
			slog.Info("updating peers after node detected connectivity issues", "id", currentNode.ID, "network", currentNode.Network)
			host, err := logic.GetHost(currentNode.HostID.String())
			if err == nil {
				if err = PublishSingleHostPeerUpdate(context.Background(), host, nil, nil); err != nil {
					slog.Warn("failed to publish update after failover peer change for node", "id", currentNode.ID, "network", currentNode.Network, "error", err)
				}
			}
		}
		slog.Debug("updated node metrics", "id", id)
	}
}

// ClientPeerUpdate  message handler -- handles updating peers after signal from client nodes
func ClientPeerUpdate(client mqtt.Client, msg mqtt.Message) {
	id, err := getID(msg.Topic())
	if err != nil {
		slog.Error("error getting node.ID sent on ", "topic", msg.Topic(), "error", err)
		return
	}
	currentNode, err := logic.GetNodeByID(id)
	if err != nil {
		slog.Error("error getting node", "id", id, "error", err)
		return
	}
	decrypted, decryptErr := decryptMsg(&currentNode, msg.Payload())
	if decryptErr != nil {
		slog.Error("failed to decrypt message for node", "id", id, "error", decryptErr)
		return
	}
	switch decrypted[0] {
	case ncutils.ACK:
		// do we still need this
	case ncutils.DONE:
		if err = PublishPeerUpdate(); err != nil {
			slog.Error("error publishing peer update for node", "id", currentNode.ID, "error", err)
			return
		}
	}

	slog.Info("sent peer updates after signal received from", "id", id)
}

func updateNodeMetrics(currentNode *models.Node, newMetrics *models.Metrics) bool {
	if newMetrics.FailoverPeers == nil {
		newMetrics.FailoverPeers = make(map[string]string)
	}
	oldMetrics, err := logic.GetMetrics(currentNode.ID.String())
	if err != nil {
		slog.Error("error finding old metrics for node", "id", currentNode.ID, "error", err)
		return false
	}
	if oldMetrics.FailoverPeers == nil {
		oldMetrics.FailoverPeers = make(map[string]string)
	}

	var attachedClients []models.ExtClient
	if currentNode.IsIngressGateway {
		clients, err := logic.GetExtClientsByID(currentNode.ID.String(), currentNode.Network)
		if err == nil {
			attachedClients = clients
		}
	}
	if len(attachedClients) > 0 {
		// associate ext clients with IDs
		for i := range attachedClients {
			extMetric := newMetrics.Connectivity[attachedClients[i].PublicKey]
			if len(extMetric.NodeName) == 0 &&
				len(newMetrics.Connectivity[attachedClients[i].ClientID].NodeName) > 0 { // cover server clients
				extMetric = newMetrics.Connectivity[attachedClients[i].ClientID]
				if extMetric.TotalReceived > 0 && extMetric.TotalSent > 0 {
					extMetric.Connected = true
				}
			}
			extMetric.NodeName = attachedClients[i].ClientID
			delete(newMetrics.Connectivity, attachedClients[i].PublicKey)
			newMetrics.Connectivity[attachedClients[i].ClientID] = extMetric
		}
	}

	// run through metrics for each peer
	for k := range newMetrics.Connectivity {
		currMetric := newMetrics.Connectivity[k]
		oldMetric := oldMetrics.Connectivity[k]
		currMetric.TotalTime += oldMetric.TotalTime
		currMetric.Uptime += oldMetric.Uptime // get the total uptime for this connection
		if currMetric.CollectedByProxy {
			currMetric.TotalReceived += oldMetric.TotalReceived
			currMetric.TotalSent += oldMetric.TotalSent
		} else {
			if currMetric.TotalReceived < oldMetric.TotalReceived {
				currMetric.TotalReceived += oldMetric.TotalReceived
			} else {
				currMetric.TotalReceived += int64(math.Abs(float64(currMetric.TotalReceived) - float64(oldMetric.TotalReceived)))
			}
			if currMetric.TotalSent < oldMetric.TotalSent {
				currMetric.TotalSent += oldMetric.TotalSent
			} else {
				currMetric.TotalSent += int64(math.Abs(float64(currMetric.TotalSent) - float64(oldMetric.TotalSent)))
			}
		}
		if currMetric.Uptime == 0 || currMetric.TotalTime == 0 {
			currMetric.PercentUp = 0
		} else {
			currMetric.PercentUp = 100.0 * (float64(currMetric.Uptime) / float64(currMetric.TotalTime))
		}
		totalUpMinutes := currMetric.Uptime * ncutils.CheckInInterval
		currMetric.ActualUptime = time.Duration(totalUpMinutes) * time.Minute
		delete(oldMetrics.Connectivity, k) // remove from old data
		newMetrics.Connectivity[k] = currMetric

	}

	// add nodes that need failover
	nodes, err := logic.GetNetworkNodes(currentNode.Network)
	if err != nil {
		slog.Error("failed to retrieve nodes while updating metrics", "error", err)
		return false
	}
	for _, node := range nodes {
		if !newMetrics.Connectivity[node.ID.String()].Connected &&
			len(newMetrics.Connectivity[node.ID.String()].NodeName) > 0 &&
			node.Connected &&
			len(node.FailoverNode) > 0 &&
			!node.Failover {
			newMetrics.FailoverPeers[node.ID.String()] = node.FailoverNode.String()
		}
	}
	shouldUpdate := len(oldMetrics.FailoverPeers) == 0 && len(newMetrics.FailoverPeers) > 0
	for k, v := range oldMetrics.FailoverPeers {
		if len(newMetrics.FailoverPeers[k]) > 0 && len(v) == 0 {
			shouldUpdate = true
		}

		if len(v) > 0 && len(newMetrics.FailoverPeers[k]) == 0 {
			newMetrics.FailoverPeers[k] = v
		}
	}

	for k := range oldMetrics.Connectivity { // cleanup any left over data, self healing
		delete(newMetrics.Connectivity, k)
	}
	return shouldUpdate
}

func handleNewNodeDNS(host *models.Host, node *models.Node) error {
	dns := models.DNSUpdate{
		Action: models.DNSInsert,
		Name:   host.Name + "." + node.Network,
	}
	if node.Address.IP != nil {
		dns.Address = node.Address.IP.String()
		if err := PublishDNSUpdate(node.Network, dns); err != nil {
			return err
		}
	} else if node.Address6.IP != nil {
		dns.Address = node.Address6.IP.String()
		if err := PublishDNSUpdate(node.Network, dns); err != nil {
			return err
		}
	}
	if err := PublishAllDNS(node); err != nil {
		return err
	}
	return nil
}

func handleHostCheckin(h, currentHost *models.Host) bool {
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
		h.DefaultInterface != currentHost.DefaultInterface
	if ifaceDelta { // only save if something changes
		currentHost.EndpointIP = h.EndpointIP
		currentHost.Interfaces = h.Interfaces
		currentHost.DefaultInterface = h.DefaultInterface
		currentHost.NatType = h.NatType
		if err := logic.UpsertHost(currentHost); err != nil {
			slog.Error("failed to update host after check-in", "name", h.Name, "id", h.ID, "error", err)
			return false
		}
		slog.Info("updated host after check-in", "name", currentHost.Name, "id", currentHost.ID)
	}

	slog.Info("check-in processed for host", "name", h.Name, "id", h.ID)
	return ifaceDelta
}
