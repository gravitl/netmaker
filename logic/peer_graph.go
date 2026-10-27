package logic

import (
	"context"
	"slices"
	"sync"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

// PeerGraph is a tenant's hosts and nodes loaded once, used to compute peer
// updates for many hosts without a DB query per (host, peer) pair.
// It is read-only after construction, apart from ext clients loaded on first
// successful use, and safe for concurrent use.
type PeerGraph struct {
	hosts map[uuid.UUID]*schema.Host
	// byNetwork holds the only copy of the node data; nodes points into it.
	byNetwork map[string][]models.Node
	nodes     map[string]*models.Node

	// ext clients by ingress gateway node ID, loaded on first successful use;
	// nil until then.
	extClientsMu sync.Mutex
	extClients   map[string][]models.ExtClient
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

// GatewayExtClients - returns a copy of the ext clients attached to the
// gateway node. The tenant's ext clients are loaded once per graph instead of
// once per (host, gateway peer).
func (g *PeerGraph) GatewayExtClients(ctx context.Context, gatewayID string) ([]models.ExtClient, error) {
	g.extClientsMu.Lock()
	defer g.extClientsMu.Unlock()
	if g.extClients == nil {
		clients, err := GetAllExtClients(ctx)
		if err != nil {
			return nil, err
		}
		g.extClients = make(map[string][]models.ExtClient)
		for i := range clients {
			gwID := clients[i].IngressGatewayID
			g.extClients[gwID] = append(g.extClients[gwID], clients[i])
		}
	}
	return slices.Clone(g.extClients[gatewayID]), nil
}

// NetworkNodes - returns a copy of the nodes in the network.
func (g *PeerGraph) NetworkNodes(network string) []models.Node {
	return slices.Clone(g.byNetwork[network])
}
