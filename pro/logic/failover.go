package logic

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"golang.org/x/exp/slog"
)

var failOverCtxMutex = &sync.RWMutex{}

func AddFailOverHook() {
	logic.HookManagerCh <- models.HookDetails{
		Hook:     FailOverCleanUpHook,
		Interval: time.Minute * 5,
	}
}

func SetFailOverCtx(failOverNode, victimNode, peerNode models.Node) error {
	failOverCtxMutex.Lock()
	defer failOverCtxMutex.Unlock()
	if peerNode.FailOverPeers == nil {
		peerNode.FailOverPeers = make(map[string]struct{})
	}
	if victimNode.FailOverPeers == nil {
		victimNode.FailOverPeers = make(map[string]struct{})
	}
	peerNode.FailOverPeers[victimNode.ID.String()] = struct{}{}
	victimNode.FailOverPeers[peerNode.ID.String()] = struct{}{}
	victimNode.FailedOverBy = failOverNode.ID
	peerNode.FailedOverBy = failOverNode.ID
	if err := logic.UpsertNode(&failOverNode); err != nil {
		return err
	}
	if err := logic.UpsertNode(&victimNode); err != nil {
		return err
	}
	if err := logic.UpsertNode(&peerNode); err != nil {
		return err
	}
	return nil
}

// GetFailOverNode - gets the host acting as failOver
func GetFailOverNode(network string, allNodes []models.Node) (models.Node, error) {
	nodes := logic.GetNetworkNodesMemory(allNodes, network)
	for _, node := range nodes {
		if node.IsFailOver {
			return node, nil
		}
	}
	return models.Node{}, errors.New("auto relay not found")
}

// FailOverExists - checks if failOver exists already in the network
func FailOverExists(network string) (failOverNode models.Node, exists bool) {
	nodes, err := logic.GetNetworkNodes(network)
	if err != nil {
		return
	}
	for _, node := range nodes {
		if node.IsFailOver {
			exists = true
			failOverNode = node
			return
		}
	}
	return
}

// ResetFailedOverPeer - removes failed over node from network peers
func ResetFailedOverPeer(failedOveredNode *models.Node) error {
	nodes, err := logic.GetNetworkNodes(failedOveredNode.Network)
	if err != nil {
		return err
	}
	failedOveredNode.FailedOverBy = uuid.Nil
	failedOveredNode.FailOverPeers = make(map[string]struct{})
	err = logic.UpsertNode(failedOveredNode)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if node.FailOverPeers == nil || node.ID == failedOveredNode.ID {
			continue
		}
		delete(node.FailOverPeers, failedOveredNode.ID.String())
		logic.UpsertNode(&node)
	}
	return nil
}

// ResetFailOver - reset failovered peers
func ResetFailOver(failOverNode *models.Node) error {
	// Unset FailedOverPeers
	nodes, err := logic.GetNetworkNodes(failOverNode.Network)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if node.FailedOverBy == failOverNode.ID {
			node.FailedOverBy = uuid.Nil
			node.FailOverPeers = make(map[string]struct{})
			logic.UpsertNode(&node)
		}
	}
	return nil
}

// GetFailOverPeerIps - adds the failedOvered peerIps by the peer
func GetFailOverPeerIps(peer, node *models.Node) []net.IPNet {
	allowedips := []net.IPNet{}
	for failOverpeerID := range node.FailOverPeers {
		failOverpeer, err := logic.GetNodeByID(failOverpeerID)
		if err == nil && failOverpeer.FailedOverBy == peer.ID {
			if failOverpeer.Address.IP != nil {
				allowed := net.IPNet{
					IP:   failOverpeer.Address.IP,
					Mask: net.CIDRMask(32, 32),
				}
				allowedips = append(allowedips, allowed)
			}
			if failOverpeer.Address6.IP != nil {
				allowed := net.IPNet{
					IP:   failOverpeer.Address6.IP,
					Mask: net.CIDRMask(128, 128),
				}
				allowedips = append(allowedips, allowed)
			}
			if failOverpeer.IsEgressGateway {
				allowedips = append(allowedips, logic.GetEgressIPs(&failOverpeer)...)
			}
			if failOverpeer.IsRelay {
				for _, id := range failOverpeer.RelayedNodes {
					rNode, _ := logic.GetNodeByID(id)
					if rNode.Address.IP != nil {
						allowed := net.IPNet{
							IP:   rNode.Address.IP,
							Mask: net.CIDRMask(32, 32),
						}
						allowedips = append(allowedips, allowed)
					}
					if rNode.Address6.IP != nil {
						allowed := net.IPNet{
							IP:   rNode.Address6.IP,
							Mask: net.CIDRMask(128, 128),
						}
						allowedips = append(allowedips, allowed)
					}
					if rNode.IsEgressGateway {
						allowedips = append(allowedips, logic.GetEgressIPs(&rNode)...)
					}
				}
			}
			// handle ingress gateway peers
			if failOverpeer.IsIngressGateway {
				extPeers, _, _, err := logic.GetExtPeers(&failOverpeer, node)
				if err != nil {
					logger.Log(2, "could not retrieve ext peers for ", peer.ID.String(), err.Error())
				}
				for _, extPeer := range extPeers {
					allowedips = append(allowedips, extPeer.AllowedIPs...)
				}
			}
		}
	}
	return allowedips
}

func CreateFailOver(node models.Node) error {
	if _, exists := FailOverExists(node.Network); exists {
		return errors.New("failover already exists in the network")
	}
	host, err := logic.GetHost(node.HostID.String())
	if err != nil {
		return err
	}
	if host.OS != models.OS_Types.Linux {
		return errors.New("only linux nodes are allowed to be set as failover")
	}
	if node.IsRelayed {
		return errors.New("relayed node cannot be set as failover")
	}
	node.IsFailOver = true
	err = logic.UpsertNode(&node)
	if err != nil {
		slog.Error("failed to upsert node", "node", node.ID.String(), "error", err)
		return err
	}
	return nil
}

// DoesFailoverAckExists - checks if ack with this id exists
func DoesFailoverAckExists(id string) bool {
	failOverCtxMutex.Lock()
	defer failOverCtxMutex.Unlock()
	_, err := database.FetchRecord(database.PEER_ACK_TABLE, id)
	return err == nil
}

// RegisterFailOverAck - registers failover ack signal
func RegisterFailOverAck(nodeid, peerid string) error {
	failOverCtxMutex.Lock()
	defer failOverCtxMutex.Unlock()
	return database.Insert(fmt.Sprintf("%s-%s", nodeid, peerid), time.Now().String(), database.PEER_ACK_TABLE)
}

// DeRegisterFailOverAck - removes the peer and node acks from DB
func DeRegisterFailOverAck(nodeid, peerid string) {
	failOverCtxMutex.Lock()
	defer failOverCtxMutex.Unlock()
	database.DeleteRecord(fmt.Sprintf("%s-%s", nodeid, peerid), database.PEER_ACK_TABLE)
	database.DeleteRecord(fmt.Sprintf("%s-%s", peerid, nodeid), database.PEER_ACK_TABLE)
}

func FailOverCleanUpHook() error {
	failOverCtxMutex.Lock()
	defer failOverCtxMutex.Unlock()
	data, err := database.FetchRecords(database.PEER_ACK_TABLE)
	if err != nil {
		return err
	}
	for key, value := range data {
		parsedTime, err := time.Parse("2006-01-02 15:04:05", value)
		if err != nil {
			continue
		}
		if time.Since(parsedTime) > time.Minute*5 {
			database.DeleteRecord(key, database.PEER_ACK_TABLE)
		}
	}
	return nil
}
