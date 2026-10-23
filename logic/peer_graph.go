package logic

import (
	"context"
	"slices"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

// PeerGraph is a tenant's hosts and nodes loaded once, used to compute peer
// updates for many hosts without a DB query per (host, peer) pair.
// It is read-only after construction and safe for concurrent use.
type PeerGraph struct {
	hosts map[uuid.UUID]*schema.Host
	// byNetwork holds the only copy of the node data; nodes points into it.
	byNetwork map[string][]models.Node
	nodes     map[string]*models.Node
}

// LoadPeerGraph - loads all hosts and nodes of the tenant in ctx.
func LoadPeerGraph(ctx context.Context) (*PeerGraph, error) {
	hosts, err := (&schema.Host{}).ListAll(ctx)
	if err != nil {
		return nil, err
	}
	nodes, err := GetAllNodes(ctx)
	if err != nil {
		return nil, err
	}
	return NewPeerGraph(hosts, nodes), nil
}

// NewPeerGraph - builds a graph from hosts and nodes the caller already loaded.
func NewPeerGraph(hosts []schema.Host, nodes []models.Node) *PeerGraph {
	g := &PeerGraph{
		hosts:     make(map[uuid.UUID]*schema.Host, len(hosts)),
		byNetwork: make(map[string][]models.Node),
		nodes:     make(map[string]*models.Node, len(nodes)),
	}
	for i := range hosts {
		host := hosts[i]
		g.hosts[host.ID] = &host
	}
	for i := range nodes {
		g.byNetwork[nodes[i].Network] = append(g.byNetwork[nodes[i].Network], nodes[i])
	}
	// index after all appends so the pointers stay valid
	for network := range g.byNetwork {
		networkNodes := g.byNetwork[network]
		for i := range networkNodes {
			g.nodes[networkNodes[i].ID.String()] = &networkNodes[i]
		}
	}
	return g
}

// Host - returns a copy of the host, loading it from the DB if it was not
// part of the graph (e.g. created after the graph was loaded).
func (g *PeerGraph) Host(ctx context.Context, hostID uuid.UUID) (*schema.Host, error) {
	if h, ok := g.hosts[hostID]; ok {
		hostCopy := *h
		return &hostCopy, nil
	}
	h := &schema.Host{ID: hostID}
	if err := h.Get(ctx); err != nil {
		return nil, err
	}
	return h, nil
}

// Node - returns a copy of the node, loading it from the DB if it was not
// part of the graph.
func (g *PeerGraph) Node(nodeID string) (models.Node, error) {
	if n, ok := g.nodes[nodeID]; ok {
		return *n, nil
	}
	return getNodeByID(nodeID)
}

// NetworkNodes - returns a copy of the nodes in the network.
func (g *PeerGraph) NetworkNodes(network string) []models.Node {
	return slices.Clone(g.byNetwork[network])
}
