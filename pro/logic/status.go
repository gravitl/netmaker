package logic

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

const statusWorkerLimit = 32

type networkStatusContext struct {
	nodeMap           map[string]models.Node
	metricsMap        map[string]*models.Metrics
	hostVersions      map[uuid.UUID]string
	defaultAclPolicy  bool
	peerSummaryStatus map[string]models.NodeStatus
}

// AddNetworkStatusToNodes computes status for all nodes using preloaded metrics and node data.
func AddNetworkStatusToNodes(nodes []models.Node) []models.Node {
	if len(nodes) == 0 {
		return nodes
	}
	ctx := buildNetworkStatusContext(nodes)
	nodesWithStatus := make([]models.Node, len(nodes))
	copy(nodesWithStatus, nodes)

	var wg sync.WaitGroup
	sem := make(chan struct{}, statusWorkerLimit)
	for i := range nodesWithStatus {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			computeNodeStatusWithContext(&nodesWithStatus[i], ctx)
		}(i)
	}
	wg.Wait()
	return nodesWithStatus
}

func buildNetworkStatusContext(nodes []models.Node) *networkStatusContext {
	ctx := &networkStatusContext{
		nodeMap:           make(map[string]models.Node, len(nodes)),
		peerSummaryStatus: make(map[string]models.NodeStatus),
	}
	metricsIDs := make(map[string]struct{})
	hostIDs := make(map[uuid.UUID]struct{})

	for _, node := range nodes {
		if node.IsStatic {
			if node.StaticNode.IngressGatewayID != "" {
				metricsIDs[node.StaticNode.IngressGatewayID] = struct{}{}
			}
			continue
		}
		id := node.ID.String()
		ctx.nodeMap[id] = node
		metricsIDs[id] = struct{}{}
		hostIDs[node.HostID] = struct{}{}
	}

	defaultPolicy, _ := logic.GetDefaultPolicy(schema.NetworkID(nodes[0].Network), models.DevicePolicy)
	ctx.defaultAclPolicy = defaultPolicy.Enabled

	metricsIDList := make([]string, 0, len(metricsIDs))
	for id := range metricsIDs {
		metricsIDList = append(metricsIDList, id)
	}
	ctx.metricsMap = GetMetricsForNodeIDs(metricsIDList)
	ctx.hostVersions = loadHostVersions(hostIDs)

	for id, node := range ctx.nodeMap {
		metrics := ctx.metricsMap[id]
		ctx.peerSummaryStatus[id] = peerSummaryStatus(node, metrics, ctx)
	}
	return ctx
}

func loadHostVersions(hostIDs map[uuid.UUID]struct{}) map[uuid.UUID]string {
	versions := make(map[uuid.UUID]string, len(hostIDs))
	if len(hostIDs) == 0 {
		return versions
	}
	idList := make([]uuid.UUID, 0, len(hostIDs))
	for id := range hostIDs {
		idList = append(idList, id)
	}
	var hosts []schema.Host
	err := db.FromContext(db.WithContext(context.TODO())).
		Model(&schema.Host{}).
		Where("id IN ?", idList).
		Find(&hosts).Error
	if err != nil {
		return versions
	}
	for _, host := range hosts {
		versions[host.ID] = host.Version
	}
	return versions
}

func computeNodeStatusWithContext(node *models.Node, ctx *networkStatusContext) {
	if node.IsStatic {
		computeStaticNodeStatus(node, ctx)
		return
	}
	if !node.Connected {
		node.Status = models.Disconnected
		return
	}
	if time.Since(node.LastCheckIn) > models.LastCheckInThreshold {
		node.Status = models.OfflineSt
		return
	}
	version, ok := ctx.hostVersions[node.HostID]
	if !ok {
		node.Status = models.UnKnown
		return
	}
	vlt, err := logic.VersionLessThan(version, "v0.30.0")
	if err != nil {
		node.Status = models.UnKnown
		return
	}
	if vlt {
		getNodeStatusOld(node)
		return
	}
	metrics := ctx.metricsMap[node.ID.String()]
	if metrics == nil || metrics.Connectivity == nil || len(metrics.Connectivity) == 0 {
		if time.Since(node.LastCheckIn) < models.LastCheckInThreshold {
			node.Status = models.OnlineSt
			return
		}
		if node.LastCheckIn.IsZero() {
			node.Status = models.OfflineSt
			return
		}
	}
	checkPeerConnectivityWithContext(node, metrics, ctx)
}

func computeStaticNodeStatus(node *models.Node, ctx *networkStatusContext) {
	if !node.StaticNode.Enabled {
		node.Status = models.OfflineSt
		return
	}
	ingNode, ok := ctx.nodeMap[node.StaticNode.IngressGatewayID]
	if !ok {
		var err error
		ingNode, err = logic.GetNodeByID(node.StaticNode.IngressGatewayID)
		if err != nil {
			node.Status = models.OfflineSt
			return
		}
	}
	if !ctx.defaultAclPolicy {
		allowed, _ := logic.IsNodeAllowedToCommunicate(*node, ingNode, false)
		if !allowed {
			node.Status = models.OnlineSt
			return
		}
	}
	ingressMetrics := ctx.metricsMap[node.StaticNode.IngressGatewayID]
	if ingressMetrics == nil || ingressMetrics.Connectivity == nil {
		node.Status = models.UnKnown
		return
	}
	if metric, ok := ingressMetrics.Connectivity[node.StaticNode.ClientID]; ok {
		if metric.Connected {
			node.Status = models.OnlineSt
		} else {
			node.Status = models.OfflineSt
		}
		return
	}
	node.Status = models.UnKnown
}

func peerSummaryStatus(node models.Node, metrics *models.Metrics, ctx *networkStatusContext) models.NodeStatus {
	if metrics == nil || metrics.Connectivity == nil {
		if time.Since(node.LastCheckIn) < models.LastCheckInThreshold {
			return models.OnlineSt
		}
		return models.OnlineSt
	}
	peerNotConnectedCnt := 0
	for peerID, metric := range metrics.Connectivity {
		peer, ok := ctx.nodeMap[peerID]
		if !ok {
			continue
		}
		if !ctx.defaultAclPolicy {
			allowed, _ := logic.IsNodeAllowedToCommunicate(node, peer, false)
			if !allowed {
				continue
			}
		}
		if time.Since(peer.LastCheckIn) > models.LastCheckInThreshold {
			continue
		}
		if metric.Connected {
			continue
		}
		peerNotConnectedCnt++
	}
	if peerNotConnectedCnt == 0 {
		return models.OnlineSt
	}
	if len(metrics.Connectivity) > 0 && peerNotConnectedCnt == len(metrics.Connectivity) {
		return models.ErrorSt
	}
	return models.WarningSt
}

func checkPeerConnectivityWithContext(node *models.Node, metrics *models.Metrics, ctx *networkStatusContext) {
	peerNotConnectedCnt := 0
	for peerID, metric := range metrics.Connectivity {
		peer, ok := ctx.nodeMap[peerID]
		if !ok {
			continue
		}
		if !ctx.defaultAclPolicy {
			allowed, _ := logic.IsNodeAllowedToCommunicate(*node, peer, false)
			if !allowed {
				continue
			}
		}
		if time.Since(peer.LastCheckIn) > models.LastCheckInThreshold {
			continue
		}
		if metric.Connected {
			continue
		}
		if summary, ok := ctx.peerSummaryStatus[peerID]; ok {
			if summary == models.ErrorSt || summary == models.WarningSt {
				continue
			}
		}
		peerNotConnectedCnt++
	}
	if peerNotConnectedCnt > len(metrics.Connectivity)/2 {
		node.Status = models.WarningSt
		return
	}
	if len(metrics.Connectivity) > 0 && peerNotConnectedCnt == len(metrics.Connectivity) {
		node.Status = models.ErrorSt
		return
	}
	node.Status = models.OnlineSt
}

func getNodeStatusOld(node *models.Node) {
	// On CE check only last check-in time
	if node.IsStatic {
		if !node.StaticNode.Enabled {
			node.Status = models.OfflineSt
			return
		}
		node.Status = models.OnlineSt
		return
	}
	if !node.Connected {
		node.Status = models.Disconnected
		return
	}
	if time.Since(node.LastCheckIn) > time.Minute*10 {
		node.Status = models.OfflineSt
		return
	}
	node.Status = models.OnlineSt
}

func GetNodeStatus(node *models.Node, defaultEnabledPolicy bool) {

	if node.IsStatic {
		if !node.StaticNode.Enabled {
			node.Status = models.OfflineSt
			return
		}
		ingNode, err := logic.GetNodeByID(node.StaticNode.IngressGatewayID)
		if err != nil {
			node.Status = models.OfflineSt
			return
		}
		if !defaultEnabledPolicy {
			allowed, _ := logic.IsNodeAllowedToCommunicate(*node, ingNode, false)
			if !allowed {
				node.Status = models.OnlineSt
				return
			}
		}
		// check extclient connection from metrics
		ingressMetrics, err := GetMetrics(node.StaticNode.IngressGatewayID)
		if err != nil || ingressMetrics == nil || ingressMetrics.Connectivity == nil {
			node.Status = models.UnKnown
			return
		}

		if metric, ok := ingressMetrics.Connectivity[node.StaticNode.ClientID]; ok {
			if metric.Connected {
				node.Status = models.OnlineSt
				return
			} else {
				node.Status = models.OfflineSt
				return
			}
		}

		node.Status = models.UnKnown
		return
	}
	if !node.Connected {
		node.Status = models.Disconnected
		return
	}
	if time.Since(node.LastCheckIn) > models.LastCheckInThreshold {
		node.Status = models.OfflineSt
		return
	}
	host := &schema.Host{
		ID: node.HostID,
	}
	err := host.Get(db.WithContext(context.TODO()))
	if err != nil {
		node.Status = models.UnKnown
		return
	}
	vlt, err := logic.VersionLessThan(host.Version, "v0.30.0")
	if err != nil {
		node.Status = models.UnKnown
		return
	}
	if vlt {
		getNodeStatusOld(node)
		return
	}
	metrics, err := logic.GetMetrics(node.ID.String())
	if err != nil {
		return
	}
	if metrics == nil || metrics.Connectivity == nil || len(metrics.Connectivity) == 0 {
		if time.Since(node.LastCheckIn) < models.LastCheckInThreshold {
			node.Status = models.OnlineSt
			return
		}
		if node.LastCheckIn.IsZero() {
			node.Status = models.OfflineSt
			return
		}
	}
	// if node.IsFailOver {
	// 	if time.Since(node.LastCheckIn) < models.LastCheckInThreshold {
	// 		node.Status = models.OnlineSt
	// 		return
	// 	}
	// }
	// If all Peers are able to reach me and and the peer is not able to reached by any peer then return online
	/* 1. FailOver Exists
		a. check connectivity to failover Node - if no connection return warning
		b. if getting failedover and still no connection to any of the peers - then show error
		c. if getting failedOver and has connections to some peers - show warning
	2. FailOver Doesn't Exist
		a. check connectivity to pu

	*/

	// failoverNode, exists := FailOverExists(node.Network)
	// if exists && failoverNode.FailedOverBy != uuid.Nil {
	// 	// check connectivity to failover Node
	// 	if metric, ok := metrics.Connectivity[failoverNode.ID.String()]; ok {
	// 		if time.Since(failoverNode.LastCheckIn) < models.LastCheckInThreshold {
	// 			if metric.Connected {
	// 				node.Status = models.OnlineSt
	// 				return
	// 			} else {
	// 				checkPeerConnectivity(node, metrics)
	// 				return
	// 			}
	// 		}
	// 	} else {
	// 		node.Status = models.OnlineSt
	// 		return
	// 	}

	// }
	checkPeerConnectivity(node, metrics, defaultEnabledPolicy)

}

func checkPeerStatus(node *models.Node, defaultAclPolicy bool) {
	peerNotConnectedCnt := 0
	metrics, err := logic.GetMetrics(node.ID.String())
	if err != nil {
		return
	}
	if metrics == nil || metrics.Connectivity == nil {
		if time.Since(node.LastCheckIn) < models.LastCheckInThreshold {
			node.Status = models.OnlineSt
			return
		}
	}
	for peerID, metric := range metrics.Connectivity {
		peer, err := logic.GetNodeByID(peerID)
		if err != nil {
			continue
		}

		if !defaultAclPolicy {
			allowed, _ := logic.IsNodeAllowedToCommunicate(*node, peer, false)
			if !allowed {
				continue
			}
		}

		if time.Since(peer.LastCheckIn) > models.LastCheckInThreshold {
			continue
		}
		if metric.Connected {
			continue
		}
		if peer.Status == models.ErrorSt {
			continue
		}
		peerNotConnectedCnt++

	}
	if peerNotConnectedCnt == 0 {
		node.Status = models.OnlineSt
		return
	}
	if len(metrics.Connectivity) > 0 && peerNotConnectedCnt == len(metrics.Connectivity) {
		node.Status = models.ErrorSt
		return
	}
	node.Status = models.WarningSt
}

func checkPeerConnectivity(node *models.Node, metrics *models.Metrics, defaultAclPolicy bool) {
	peerNotConnectedCnt := 0
	for peerID, metric := range metrics.Connectivity {
		peer, err := logic.GetNodeByID(peerID)
		if err != nil {
			continue
		}

		if !defaultAclPolicy {
			allowed, _ := logic.IsNodeAllowedToCommunicate(*node, peer, false)
			if !allowed {
				continue
			}
		}

		if time.Since(peer.LastCheckIn) > models.LastCheckInThreshold {
			continue
		}
		if metric.Connected {
			continue
		}
		// check if peer is in error state
		checkPeerStatus(&peer, defaultAclPolicy)
		if peer.Status == models.ErrorSt || peer.Status == models.WarningSt {
			continue
		}
		peerNotConnectedCnt++

	}

	if peerNotConnectedCnt > len(metrics.Connectivity)/2 {
		node.Status = models.WarningSt
		return
	}

	if len(metrics.Connectivity) > 0 && peerNotConnectedCnt == len(metrics.Connectivity) {
		node.Status = models.ErrorSt
		return
	}

	node.Status = models.OnlineSt

}
