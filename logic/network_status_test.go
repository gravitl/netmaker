package logic

import (
	"context"
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

func TestNodeNetworkStatusKindAndFlags(t *testing.T) {
	newNode := func() *models.Node {
		n := &models.Node{}
		n.ID = uuid.New()
		return n
	}
	eli := []schema.Egress{
		{Status: true, Range: "10.0.0.0/24", Tags: datatypes.JSONMap{"routers": struct{}{}}},
	}

	allRoles := newNode()
	allRoles.IsGw = true
	allRoles.Tags = map[models.TagID]struct{}{"routers": {}}
	relay := newNode()
	relay.IsRelay = true
	egress := newNode()
	egress.Tags = map[models.TagID]struct{}{"routers": {}}
	user := newNode()
	user.OwnerID = "alice"
	plain := newNode()
	inetRouters := map[string]struct{}{allRoles.ID.String(): {}}

	tests := []struct {
		name                    string
		node                    *models.Node
		kind                    models.NetworkNodeKind
		gateway, inetGw, egress bool
	}{
		{"gateway, internet gateway and egress", allRoles, models.NetworkNodeKindNode, true, true, true},
		{"relay is a gateway", relay, models.NetworkNodeKindNode, true, false, false},
		{"egress via tag", egress, models.NetworkNodeKindNode, false, false, true},
		{"user device", user, models.NetworkNodeKindUser, false, false, false},
		{"plain node", plain, models.NetworkNodeKindNode, false, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nodeNetworkStatus(context.Background(), tt.node, schema.Host{}, eli, inetRouters, nil, nil, false)
			if got.Kind != tt.kind || got.IsGateway != tt.gateway || got.IsInternetGateway != tt.inetGw || got.IsEgress != tt.egress {
				t.Fatalf("got kind=%q gateway=%v internet_gateway=%v egress=%v, want kind=%q gateway=%v internet_gateway=%v egress=%v",
					got.Kind, got.IsGateway, got.IsInternetGateway, got.IsEgress, tt.kind, tt.gateway, tt.inetGw, tt.egress)
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
