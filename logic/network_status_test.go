package logic

import (
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func TestPeerRelayNodeID(t *testing.T) {
	nodeID, peerID, relayID, autoRelayID := uuid.New(), uuid.New(), uuid.New().String(), uuid.New().String()
	newNode := func(id uuid.UUID) *models.Node {
		n := &models.Node{}
		n.ID = id
		return n
	}

	tests := []struct {
		name  string
		setup func(node, peer *models.Node)
		want  string
	}{
		{"direct", func(node, peer *models.Node) {}, ""},
		{"auto relayed", func(node, peer *models.Node) {
			node.AutoRelayedPeers = map[string]string{peerID.String(): autoRelayID}
		}, autoRelayID},
		{"node relayed", func(node, peer *models.Node) {
			node.IsRelayed, node.RelayedBy = true, relayID
		}, relayID},
		{"peer relayed", func(node, peer *models.Node) {
			peer.IsRelayed, peer.RelayedBy = true, relayID
		}, relayID},
		{"node relayed by the peer itself", func(node, peer *models.Node) {
			node.IsRelayed, node.RelayedBy = true, peerID.String()
		}, ""},
		{"peer relayed by the node itself", func(node, peer *models.Node) {
			peer.IsRelayed, peer.RelayedBy = true, nodeID.String()
		}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, peer := newNode(nodeID), newNode(peerID)
			tt.setup(node, peer)
			if got := peerRelayNodeID(node, peer); got != tt.want {
				t.Fatalf("peerRelayNodeID() = %q, want %q", got, tt.want)
			}
		})
	}

	if got := peerRelayNodeID(newNode(nodeID), nil); got != "" {
		t.Fatalf("peerRelayNodeID() with unknown peer = %q, want empty", got)
	}
}

func TestNodeRole(t *testing.T) {
	egressTagged := func() *models.Node {
		n := &models.Node{}
		n.ID = uuid.New()
		n.Tags = map[models.TagID]struct{}{"routers": {}}
		return n
	}
	eli := []schema.Egress{
		{Status: true, Range: "10.0.0.0/24", Tags: datatypes.JSONMap{"routers": struct{}{}}},
	}

	inetGw := egressTagged()
	inetGw.IsGw = true
	gw := egressTagged()
	gw.IsGw = true
	relay := &models.Node{}
	relay.ID = uuid.New()
	relay.IsRelay = true
	egress := egressTagged()
	user := &models.Node{OwnerID: "alice"}
	user.ID = uuid.New()
	plain := &models.Node{}
	plain.ID = uuid.New()
	inetRouters := map[string]struct{}{inetGw.ID.String(): {}}

	tests := []struct {
		name string
		node *models.Node
		want models.NetworkNodeRole
	}{
		{"internet gateway wins over gateway and egress", inetGw, models.NetworkNodeRoleInternetGateway},
		{"gateway wins over egress", gw, models.NetworkNodeRoleGateway},
		{"relay is a gateway", relay, models.NetworkNodeRoleGateway},
		{"egress via tag", egress, models.NetworkNodeRoleEgress},
		{"user device", user, models.NetworkNodeRoleUser},
		{"plain node", plain, models.NetworkNodeRoleNode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nodeRole(tt.node, eli, inetRouters); got != tt.want {
				t.Fatalf("nodeRole() = %q, want %q", got, tt.want)
			}
		})
	}

	eli[0].Status = false
	if routesEgress(egress, eli) {
		t.Fatal("routesEgress() should ignore inactive egress")
	}
}

func TestCountNetworkNodeStatus(t *testing.T) {
	var s models.NetworkStatusSummary
	for _, st := range []schema.NodeStatus{schema.OnlineSt, schema.OnlineSt, schema.OfflineSt, schema.WarningSt,
		schema.ErrorSt, schema.Disconnected, schema.UnKnown, ""} {
		countNetworkNodeStatus(&s, st)
	}
	want := models.NetworkStatusSummary{Total: 8, Online: 2, Offline: 1, Warning: 1, Error: 1, Disconnected: 1, Unknown: 2}
	if s != want {
		t.Fatalf("summary = %+v, want %+v", s, want)
	}
}
