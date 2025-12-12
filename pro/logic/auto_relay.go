package logic

import (
	"context"
	"errors"
	"net"
	"sync"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
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
func SetAutoRelay(node *models.Node) {
	node.IsAutoRelay = true
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
	autoRelayNodeIDPeerNode, peerHasAutoRelayed := peerNode.AutoRelayedPeers[victimNode.ID.String()]
	if peerNode.Mutex != nil {
		peerNode.Mutex.Unlock()
	}
	if victimNode.Mutex != nil {
		victimNode.Mutex.Lock()
	}
	autoRelayNodeIDVictim, victimHasAutoRelayed := victimNode.AutoRelayedPeers[peerNode.ID.String()]
	if victimNode.Mutex != nil {
		victimNode.Mutex.Unlock()
	}
	if peerHasAutoRelayed && victimHasAutoRelayed && autoRelayNodeIDVictim == autoRelayNodeIDPeerNode {
		return errors.New("auto relay ctx is already set")
	}
	return nil
}
func SetAutoRelayCtx(autoRelayNode, victimNode, peerNode models.Node) error {
	autoRelayCtxMutex.Lock()
	defer autoRelayCtxMutex.Unlock()
	if peerNode.AutoRelayedPeers == nil {
		peerNode.AutoRelayedPeers = make(map[string]string)
	}
	if victimNode.AutoRelayedPeers == nil {
		victimNode.AutoRelayedPeers = make(map[string]string)
	}
	if peerNode.Mutex != nil {
		peerNode.Mutex.Lock()
	}
	autoRelayNodeIDPeerNode, peerHasAutoRelayed := peerNode.AutoRelayedPeers[victimNode.ID.String()]
	if peerNode.Mutex != nil {
		peerNode.Mutex.Unlock()
	}
	if victimNode.Mutex != nil {
		victimNode.Mutex.Lock()
	}
	autoRelayNodeIDVictim, victimHasAutoRelayed := victimNode.AutoRelayedPeers[peerNode.ID.String()]
	if victimNode.Mutex != nil {
		victimNode.Mutex.Unlock()
	}
	if peerHasAutoRelayed && victimHasAutoRelayed && autoRelayNodeIDVictim == autoRelayNodeIDPeerNode {
		return errors.New("auto relay ctx is already set")
	}
	if peerNode.Mutex != nil {
		peerNode.Mutex.Lock()
	}
	peerNode.AutoRelayedPeers[victimNode.ID.String()] = autoRelayNode.ID.String()
	if peerNode.Mutex != nil {
		peerNode.Mutex.Unlock()
	}
	if victimNode.Mutex != nil {
		victimNode.Mutex.Lock()
	}
	victimNode.AutoRelayedPeers[peerNode.ID.String()] = autoRelayNode.ID.String()
	if victimNode.Mutex != nil {
		victimNode.Mutex.Unlock()
	}
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
func DoesAutoRelayExist(network string) (autoRelayNodes []models.Node) {
	autoRelayCacheMutex.RLock()
	defer autoRelayCacheMutex.RUnlock()
	if !servercfg.CacheEnabled() {
		nodes, _ := logic.GetNetworkNodes(network)
		for _, node := range nodes {
			if node.IsAutoRelay {
				autoRelayNodes = append(autoRelayNodes, node)
			}
		}
	}
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
	autoRelayedNode.AutoRelayedPeers = make(map[string]string)
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
		for autoRelayedPeerID, autoRelayID := range node.AutoRelayedPeers {
			if autoRelayID != autoRelayNode.ID.String() {
				continue
			}
			delete(node.AutoRelayedPeers, autoRelayedPeerID)
			logic.UpsertNode(&node)
			peer, err := logic.GetNodeByID(autoRelayedPeerID)
			if err == nil {
				delete(peer.AutoRelayedPeers, node.ID.String())
				logic.UpsertNode(&peer)
			}
		}
	}
	return nil
}

// GetAutoRelayPeerIps - adds the autorelayed peerIps by the peer
func GetAutoRelayPeerIps(peer, node *models.Node) []net.IPNet {
	allowedips := []net.IPNet{}
	eli, _ := (&schema.Egress{Network: node.Network}).ListByNetwork(db.WithContext(context.TODO()))
	acls, _ := logic.ListAclsByNetwork(models.NetworkID(node.Network))
	for autoRelayedpeerID, autoRelayID := range node.AutoRelayedPeers {
		if peer.ID.String() != autoRelayID {
			continue
		}
		autoRelayedpeer, err := logic.GetNodeByID(autoRelayedpeerID)
		if err == nil {
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
				extPeers, _, _, err := logic.GetExtPeers(&autoRelayedpeer, node, make(map[string]models.PeerIdentity))
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
	if servercfg.CacheEnabled() {
		SetAutoRelayInCache(node)
	}
	return nil
}
