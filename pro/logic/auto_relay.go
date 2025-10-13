package logic

import (
	"context"
	"errors"
	"net"
	"sync"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"golang.org/x/exp/slog"
)

var autoRelayCtxMutex = &sync.RWMutex{}
var autoRelayCacheMutex = &sync.RWMutex{}
var autoRelayCache = make(map[models.NetworkID][]string)

func InitAutoRelayCache() {
	autoRelayCacheMutex.Lock()
	defer autoRelayCacheMutex.Unlock()
	allNodes, err := logic.GetAllNodes()
	if err != nil {
		return
	}
	for _, node := range allNodes {
		if node.IsAutoRelay {
			autoRelayCache[models.NetworkID(node.Network)] = append(autoRelayCache[models.NetworkID(node.Network)], node.ID.String())
		}
	}

}

func CheckAutoRelayCtx(autoRelayNode, victimNode, peerNode models.Node) error {
	autoRelayCtxMutex.RLock()
	defer autoRelayCtxMutex.RUnlock()
	if peerNode.AutoRelayedPeers == nil {
		return nil
	}
	if victimNode.AutoRelayedPeers == nil {
		return nil
	}
	if peerNode.Mutex != nil {
		peerNode.Mutex.Lock()
	}
	_, peerHasAutoRelayed := peerNode.AutoRelayedPeers[victimNode.ID.String()]
	if peerNode.Mutex != nil {
		peerNode.Mutex.Unlock()
	}
	if victimNode.Mutex != nil {
		victimNode.Mutex.Lock()
	}
	_, victimHasAutoRelayed := victimNode.AutoRelayedPeers[peerNode.ID.String()]
	if victimNode.Mutex != nil {
		victimNode.Mutex.Unlock()
	}
	if peerHasAutoRelayed && victimHasAutoRelayed &&
		victimNode.AutoRelayedBy == autoRelayNode.ID && peerNode.AutoRelayedBy == autoRelayNode.ID {
		return errors.New("auto relay ctx is already set")
	}
	return nil
}
func SetAutoRelayCtx(autoRelayNode, victimNode, peerNode models.Node) error {
	autoRelayCtxMutex.Lock()
	defer autoRelayCtxMutex.Unlock()
	if peerNode.AutoRelayedPeers == nil {
		peerNode.AutoRelayedPeers = make(map[string]struct{})
	}
	if victimNode.AutoRelayedPeers == nil {
		victimNode.AutoRelayedPeers = make(map[string]struct{})
	}
	if peerNode.Mutex != nil {
		peerNode.Mutex.Lock()
	}
	_, peerHasAutoRelayed := peerNode.AutoRelayedPeers[victimNode.ID.String()]
	if peerNode.Mutex != nil {
		peerNode.Mutex.Unlock()
	}
	if victimNode.Mutex != nil {
		victimNode.Mutex.Lock()
	}
	_, victimHasAutoRelayed := victimNode.AutoRelayedPeers[peerNode.ID.String()]
	if victimNode.Mutex != nil {
		victimNode.Mutex.Unlock()
	}
	if peerHasAutoRelayed && victimHasAutoRelayed &&
		victimNode.AutoRelayedBy == autoRelayNode.ID && peerNode.AutoRelayedBy == autoRelayNode.ID {
		return errors.New("auto relay ctx is already set")
	}
	if peerNode.Mutex != nil {
		peerNode.Mutex.Lock()
	}
	peerNode.AutoRelayedPeers[victimNode.ID.String()] = struct{}{}
	if peerNode.Mutex != nil {
		peerNode.Mutex.Unlock()
	}
	if victimNode.Mutex != nil {
		victimNode.Mutex.Lock()
	}
	victimNode.AutoRelayedPeers[peerNode.ID.String()] = struct{}{}
	if victimNode.Mutex != nil {
		victimNode.Mutex.Unlock()
	}
	victimNode.AutoRelayedBy = autoRelayNode.ID
	peerNode.AutoRelayedBy = autoRelayNode.ID
	if err := logic.UpsertNode(&victimNode); err != nil {
		return err
	}
	if err := logic.UpsertNode(&peerNode); err != nil {
		return err
	}
	return nil
}

// GetAutoRelayNode - gets the host acting as autoRelay
func GetAutoRelayNode(network string, allNodes []models.Node) (models.Node, error) {
	nodes := logic.GetNetworkNodesMemory(allNodes, network)
	for _, node := range nodes {
		if node.IsAutoRelay {
			return node, nil
		}
	}
	return models.Node{}, errors.New("auto relay not found")
}

func RemoveAutoRelayFromCache(network string) {
	autoRelayCacheMutex.Lock()
	defer autoRelayCacheMutex.Unlock()
	delete(autoRelayCache, models.NetworkID(network))
}

func SetAutoRelayInCache(node models.Node) {
	autoRelayCacheMutex.Lock()
	defer autoRelayCacheMutex.Unlock()
	autoRelayCache[models.NetworkID(node.Network)] = append(autoRelayCache[models.NetworkID(node.Network)], node.ID.String())
}

// DoesAutoRelayExist - checks if autorelay exists already in the network
func DoesAutoRelayExist(network string) (autoRelayNodes []models.Node, exists bool) {
	autoRelayCacheMutex.RLock()
	defer autoRelayCacheMutex.RUnlock()
	if nodeIDs, ok := autoRelayCache[models.NetworkID(network)]; ok {
		for _, nodeID := range nodeIDs {
			autoRelayNode, err := logic.GetNodeByID(nodeID)
			if err == nil {
				autoRelayNodes = append(autoRelayNodes, autoRelayNode)
			}
		}

	}
	return
}

// ResetAutoRelayedPeer - removes auto relayed over node from network peers
func ResetAutoRelayedPeer(autoRelayedNode *models.Node) error {
	nodes, err := logic.GetNetworkNodes(autoRelayedNode.Network)
	if err != nil {
		return err
	}
	autoRelayedNode.AutoRelayedBy = uuid.Nil
	autoRelayedNode.AutoRelayedPeers = make(map[string]struct{})
	err = logic.UpsertNode(autoRelayedNode)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if node.AutoRelayedPeers == nil || node.ID == autoRelayedNode.ID {
			continue
		}
		delete(node.AutoRelayedPeers, autoRelayedNode.ID.String())
		logic.UpsertNode(&node)
	}
	return nil
}

// ResetAutoRelay - reset autorelayed peers
func ResetAutoRelay(autoRelayNode *models.Node) error {
	// Unset autorelayed peers
	nodes, err := logic.GetNetworkNodes(autoRelayNode.Network)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if node.AutoRelayedBy == autoRelayNode.ID {
			node.AutoRelayedBy = uuid.Nil
			node.AutoRelayedPeers = make(map[string]struct{})
			logic.UpsertNode(&node)
		}
	}
	return nil
}

// GetAutoRelayPeerIps - adds the autorelayed peerIps by the peer
func GetAutoRelayPeerIps(peer, node *models.Node) []net.IPNet {
	allowedips := []net.IPNet{}
	eli, _ := (&schema.Egress{Network: node.Network}).ListByNetwork(db.WithContext(context.TODO()))
	acls, _ := logic.ListAclsByNetwork(models.NetworkID(node.Network))
	for autoRelayedpeerID := range node.AutoRelayedPeers {
		autoRelayedpeer, err := logic.GetNodeByID(autoRelayedpeerID)
		if err == nil && autoRelayedpeer.AutoRelayedBy == peer.ID {
			logic.GetNodeEgressInfo(&autoRelayedpeer, eli, acls)
			if autoRelayedpeer.Address.IP != nil {
				allowed := net.IPNet{
					IP:   autoRelayedpeer.Address.IP,
					Mask: net.CIDRMask(32, 32),
				}
				allowedips = append(allowedips, allowed)
			}
			if autoRelayedpeer.Address6.IP != nil {
				allowed := net.IPNet{
					IP:   autoRelayedpeer.Address6.IP,
					Mask: net.CIDRMask(128, 128),
				}
				allowedips = append(allowedips, allowed)
			}
			if autoRelayedpeer.EgressDetails.IsEgressGateway {
				allowedips = append(allowedips, logic.GetEgressIPs(&autoRelayedpeer)...)
			}
			if autoRelayedpeer.IsRelay {
				for _, id := range autoRelayedpeer.RelayedNodes {
					rNode, _ := logic.GetNodeByID(id)
					logic.GetNodeEgressInfo(&rNode, eli, acls)
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
					if rNode.EgressDetails.IsEgressGateway {
						allowedips = append(allowedips, logic.GetEgressIPs(&rNode)...)
					}
				}
			}
			// handle ingress gateway peers
			if autoRelayedpeer.IsIngressGateway {
				extPeers, _, _, err := logic.GetExtPeers(&autoRelayedpeer, node)
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

func CreateAutoRelay(node models.Node) error {
	if _, exists := DoesAutoRelayExist(node.Network); exists {
		return errors.New("autorelay already exists in the network")
	}
	host, err := logic.GetHost(node.HostID.String())
	if err != nil {
		return err
	}
	if host.OS != models.OS_Types.Linux {
		return errors.New("only linux nodes are allowed to be set as autoRelay")
	}
	if node.IsRelayed {
		return errors.New("relayed node cannot be set as autoRelay")
	}
	node.IsAutoRelay = true
	err = logic.UpsertNode(&node)
	if err != nil {
		slog.Error("failed to upsert node", "node", node.ID.String(), "error", err)
		return err
	}
	SetAutoRelayInCache(node)
	return nil
}
