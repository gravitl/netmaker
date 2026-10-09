package logic

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func TestPeerRelayNodeID(t *testing.T) {
	nodeID, peerID, relayID, peerRelayID, autoRelayID := uuid.New(), uuid.New(), uuid.New().String(), uuid.New().String(), uuid.New().String()
	newNode := func(id uuid.UUID) *models.Node {
		n := &models.Node{}
		n.ID = id
		return n
	}

	tests := []struct {
		name            string
		setup           func(node, peer *models.Node)
		ignoreRelayedBy string
		want            string
	}{
		{"direct", func(node, peer *models.Node) {}, "", ""},
		{"auto relayed", func(node, peer *models.Node) {
			node.AutoRelayedPeers = map[string]string{peerID.String(): autoRelayID}
		}, "", autoRelayID},
		{"node relayed", func(node, peer *models.Node) {
			node.IsRelayed, node.RelayedBy = true, relayID
		}, "", relayID},
		{"peer relayed", func(node, peer *models.Node) {
			peer.IsRelayed, peer.RelayedBy = true, relayID
		}, "", relayID},
		{"peer relay wins over viewer exit hairpin", func(node, peer *models.Node) {
			node.IsRelayed, node.RelayedBy = true, relayID
			peer.IsRelayed, peer.RelayedBy = true, peerRelayID
		}, "", peerRelayID},
		{"node relayed by the peer itself", func(node, peer *models.Node) {
			node.IsRelayed, node.RelayedBy = true, peerID.String()
		}, "", ""},
		{"peer relayed by the node itself", func(node, peer *models.Node) {
			peer.IsRelayed, peer.RelayedBy = true, nodeID.String()
		}, "", ""},
		{"ignore exit RelayedBy on node", func(node, peer *models.Node) {
			node.IsRelayed, node.RelayedBy = true, relayID
		}, relayID, ""},
		{"ignore exit RelayedBy on peer", func(node, peer *models.Node) {
			peer.IsRelayed, peer.RelayedBy = true, relayID
		}, relayID, ""},
		{"auto relay wins over ignore", func(node, peer *models.Node) {
			node.IsRelayed, node.RelayedBy = true, relayID
			node.AutoRelayedPeers = map[string]string{peerID.String(): autoRelayID}
		}, relayID, autoRelayID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, peer := newNode(nodeID), newNode(peerID)
			tt.setup(node, peer)
			if got := peerRelayNodeID(node, peer, tt.ignoreRelayedBy); got != tt.want {
				t.Fatalf("peerRelayNodeID() = %q, want %q", got, tt.want)
			}
		})
	}

	if got := peerRelayNodeID(newNode(nodeID), nil, ""); got != "" {
		t.Fatalf("peerRelayNodeID() with unknown peer = %q, want empty", got)
	}
}

func newTestNode() *models.Node {
	n := &models.Node{}
	n.ID = uuid.New()
	return n
}

// newTestStatusBuilder builds a builder over the given nodes with every egress
// accessible, so tests do not need policies in the database.
func newTestStatusBuilder(eli []schema.Egress, opts NetworkStatusOptions, nodes ...*models.Node) *networkStatusBuilder {
	b := &networkStatusBuilder{
		ctx:               context.Background(),
		opts:              opts,
		eli:               eli,
		inetRouters:       InternetEgressRoutingNodeIDsFromList(eli),
		hosts:             map[string]schema.Host{},
		names:             map[string]string{},
		nodesByID:         map[string]*models.Node{},
		users:             map[string]*schema.User{},
		relayConnectivity: map[string]map[string]models.Metric{},
		deviceDefault:     true,
		userDefault:       true,
	}
	for _, n := range nodes {
		b.nodesByID[n.ID.String()] = n
		b.names[n.ID.String()] = "host-" + n.ID.String()[:4]
	}
	b.indexEgressRouters()
	return b
}

// stubMetrics makes GetMetrics return the given connectivity per node ID.
func stubMetrics(t *testing.T, byNode map[string]map[string]models.Metric) {
	orig := GetMetrics
	GetMetrics = func(_ context.Context, nodeID string) (*models.Metrics, error) {
		return &models.Metrics{NodeID: nodeID, Connectivity: byNode[nodeID]}, nil
	}
	t.Cleanup(func() { GetMetrics = orig })
}

func TestNodeNetworkStatusKindAndFlags(t *testing.T) {
	stubMetrics(t, nil)
	eli := []schema.Egress{
		{ID: "lan", Status: true, Range: "10.0.0.0/24", Tags: datatypes.JSONMap{"routers": json.Number("10")}},
	}

	allRoles := newTestNode()
	allRoles.IsGw = true
	allRoles.Tags = map[models.TagID]struct{}{"routers": {}}
	relay := newTestNode()
	relay.IsRelay = true
	egress := newTestNode()
	egress.Tags = map[models.TagID]struct{}{"routers": {}}
	user := newTestNode()
	user.OwnerID = "alice"
	plain := newTestNode()

	b := newTestStatusBuilder(eli, NetworkStatusOptions{}, allRoles, relay, egress, user, plain)
	b.inetRouters[allRoles.ID.String()] = struct{}{}

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
			got := b.nodeNetworkStatus(tt.node)
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

func TestIndexEgressRouters(t *testing.T) {
	direct, tagged, other := newTestNode(), newTestNode(), newTestNode()
	tagged.Tags = map[models.TagID]struct{}{"routers": {}}
	eli := []schema.Egress{
		{ID: "lan", Status: true, Range: "10.0.0.0/24",
			Nodes: datatypes.JSONMap{direct.ID.String(): json.Number("20"), "deleted-node": json.Number("1")},
			Tags:  datatypes.JSONMap{"routers": json.Number("5")}},
		{ID: "off", Status: false, Range: "10.1.0.0/24", Nodes: datatypes.JSONMap{direct.ID.String(): json.Number("1")}},
		{ID: "inet", Status: true, Type: schema.EgressTypeInternet, Nodes: datatypes.JSONMap{direct.ID.String(): json.Number("1")}},
		{ID: "no-routers", Status: true, Range: "10.2.0.0/24"},
	}
	b := newTestStatusBuilder(eli, NetworkStatusOptions{}, direct, tagged, other)

	if len(b.egresses) != 2 || b.egresses[0].ID != "lan" || b.egresses[1].ID != "inet" {
		t.Fatalf("egresses = %v, want lan and inet", b.egresses)
	}
	want := []egressRouter{{tagged.ID.String(), 5}, {direct.ID.String(), 20}}
	got := b.routers["lan"]
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("routers = %v, want %v", got, want)
	}
	if got := b.routers["inet"]; len(got) != 1 || got[0].nodeID != direct.ID.String() {
		t.Fatalf("internet routers = %v, want only %s", got, direct.ID)
	}
}

func TestNodeNetworkStatusEgresses(t *testing.T) {
	node, primary, backup, gw, exit := newTestNode(), newTestNode(), newTestNode(), newTestNode(), newTestNode()
	eli := []schema.Egress{
		{ID: "lan", Name: "office", Status: true, Range: "10.0.0.0/24",
			Nodes: datatypes.JSONMap{primary.ID.String(): json.Number("10"), backup.ID.String(): json.Number("20")}},
		{ID: "inet", Status: true, Type: schema.EgressTypeInternet, Nodes: datatypes.JSONMap{exit.ID.String(): json.Number("1")}},
	}
	connectivity := map[string]models.Metric{
		primary.ID.String(): {Connected: false, Latency: 999},
		backup.ID.String():  {Connected: true, Latency: 12, PercentUp: 99.5},
	}
	stubMetrics(t, map[string]map[string]models.Metric{node.ID.String(): connectivity})

	t.Run("failover to connected backup router", func(t *testing.T) {
		b := newTestStatusBuilder(eli, NetworkStatusOptions{IncludeEgress: true}, node, primary, backup)
		got := b.nodeNetworkStatus(node)
		if got.TotalEgresses != 1 || got.ConnectedEgresses != 1 || len(got.Egresses) != 1 {
			t.Fatalf("egress counts = %d/%d, list = %v", got.ConnectedEgresses, got.TotalEgresses, got.Egresses)
		}
		e := got.Egresses[0]
		if e.EgressID != "lan" || e.Name != "office" || len(e.Ranges) != 1 || e.Ranges[0] != "10.0.0.0/24" {
			t.Fatalf("egress identity = %+v", e)
		}
		if e.RoutingNode.ID != backup.ID.String() || e.RoutingNodesTotal != 2 || e.RoutingNodesConnected != 1 {
			t.Fatalf("routing = %s %d/%d, want backup 1/2", e.RoutingNode.ID, e.RoutingNodesConnected, e.RoutingNodesTotal)
		}
		if !e.Connected || e.LatencyMs != 12 || e.PercentUp != 99.5 || e.IsRelayed {
			t.Fatalf("link = %+v", e)
		}
	})

	t.Run("routing node does not list its own egress", func(t *testing.T) {
		b := newTestStatusBuilder(eli, NetworkStatusOptions{IncludeEgress: true}, node, primary, backup)
		if got := b.nodeNetworkStatus(primary); got.TotalEgresses != 0 {
			t.Fatalf("routing node egresses = %d, want 0", got.TotalEgresses)
		}
	})

	t.Run("relayed through gateway", func(t *testing.T) {
		relayed := *node
		relayed.IsRelayed, relayed.RelayedBy = true, gw.ID.String()
		b := newTestStatusBuilder(eli, NetworkStatusOptions{IncludeEgress: true}, &relayed, primary, backup, gw)
		e := b.nodeNetworkStatus(&relayed).Egresses[0]
		if !e.IsRelayed || (e.Via == nil || e.Via.ID != gw.ID.String()) {
			t.Fatalf("link = %+v, want relayed via gateway", e)
		}
	})

	t.Run("exit node relays egress unless it bypasses", func(t *testing.T) {
		exitClient := *node
		exitClient.SelectedInternetEgressID = "inet"
		// An active exit relays the node through the exit routing node (IGW client).
		exitClient.IsRelayed, exitClient.RelayedBy, exitClient.InternetGwID = true, exit.ID.String(), exit.ID.String()

		// The node reaches the exit at 30ms and backup/primary; the exit reaches
		// primary at 4ms and backup at 5ms, so primary wins via the exit.
		stubMetrics(t, map[string]map[string]models.Metric{
			node.ID.String(): {
				exit.ID.String():    {Connected: true, Latency: 30, PercentUp: 97},
				primary.ID.String(): {Connected: true, Latency: 15, PercentUp: 99},
				backup.ID.String():  {Connected: true, Latency: 12, PercentUp: 99.5},
				gw.ID.String():      {Connected: true, Latency: 8, PercentUp: 100},
			},
			exit.ID.String(): {
				primary.ID.String(): {Connected: true, Latency: 4, PercentUp: 95},
				backup.ID.String():  {Connected: true, Latency: 5, PercentUp: 99},
			},
			gw.ID.String(): {
				backup.ID.String(): {Connected: true, Latency: 6, PercentUp: 90},
			},
		})

		b := newTestStatusBuilder(eli, NetworkStatusOptions{IncludeEgress: true, IncludePeers: true}, &exitClient, primary, backup, exit, gw)
		got := b.nodeNetworkStatus(&exitClient)
		e := got.Egresses[0]
		if !e.IsRelayed || (e.Via == nil || e.Via.ID != exit.ID.String()) {
			t.Fatalf("without bypass: link = %+v, want relayed via exit", e)
		}
		if e.RoutingNode.ID != primary.ID.String() || e.RoutingNodesConnected != 2 || !e.Connected ||
			e.LatencyMs != 34 || e.PercentUp != 95 {
			t.Fatalf("without bypass: path = %+v, want primary via exit at 34ms", e)
		}
		for _, p := range got.Peers {
			if p.PeerID == exit.ID.String() {
				if p.IsRelayed {
					t.Fatalf("without bypass: exit peer should not be relayed, got %+v", p)
				}
				continue
			}
			if !p.IsRelayed || p.Via == nil || p.Via.ID != exit.ID.String() {
				t.Fatalf("without bypass: peer %s = %+v, want relayed via exit", p.PeerID, p)
			}
		}

		bypassing := append([]schema.Egress(nil), eli...)
		bypassing[1].BypassEgressRoutes = true
		b = newTestStatusBuilder(bypassing, NetworkStatusOptions{IncludeEgress: true, IncludePeers: true}, &exitClient, primary, backup, exit, gw)
		got = b.nodeNetworkStatus(&exitClient)
		e = got.Egresses[0]
		if e.IsRelayed || e.Via != nil {
			t.Fatalf("with bypass: egress link = %+v, want direct", e)
		}
		if e.RoutingNode.ID != primary.ID.String() || !e.Connected || e.LatencyMs != 15 {
			t.Fatalf("with bypass: path = %+v, want primary direct at 15ms", e)
		}
		// The exit itself is still reached directly for internet traffic.
		if inet := got.Egresses[1]; !inet.IsInternet || inet.IsRelayed || inet.LatencyMs != 30 {
			t.Fatalf("with bypass: internet egress = %+v, want direct to exit at 30ms", inet)
		}
		// With bypass: site-egress routers are direct; plain gateway still via exit.
		for _, p := range got.Peers {
			switch p.PeerID {
			case exit.ID.String(), primary.ID.String(), backup.ID.String():
				if p.IsRelayed {
					t.Fatalf("with bypass: peer %s = %+v, want direct (exit/egress)", p.PeerID, p)
				}
			case gw.ID.String():
				if !p.IsRelayed || p.Via == nil || p.Via.ID != exit.ID.String() {
					t.Fatalf("with bypass: plain gateway peer = %+v, want via exit", p)
				}
			}
		}

		// Alternate exit routers (not RelayedBy) stay direct for exit clients.
		altExit := newTestNode()
		eliAlt := append([]schema.Egress(nil), bypassing...)
		eliAlt[1].Nodes = datatypes.JSONMap{
			exit.ID.String():    json.Number("1"),
			altExit.ID.String(): json.Number("1"),
		}
		stubMetrics(t, map[string]map[string]models.Metric{
			node.ID.String(): {
				exit.ID.String():    {Connected: true, Latency: 30, PercentUp: 97},
				altExit.ID.String(): {Connected: true, Latency: 126, PercentUp: 80},
				primary.ID.String(): {Connected: true, Latency: 15, PercentUp: 99},
				gw.ID.String():      {Connected: true, Latency: 8, PercentUp: 100},
			},
		})
		b = newTestStatusBuilder(eliAlt, NetworkStatusOptions{IncludePeers: true}, &exitClient, primary, exit, altExit, gw)
		got = b.nodeNetworkStatus(&exitClient)
		for _, p := range got.Peers {
			switch p.PeerID {
			case altExit.ID.String(), exit.ID.String():
				if p.IsRelayed {
					t.Fatalf("alternate exit peer %s = %+v, want direct", p.PeerID, p)
				}
			case gw.ID.String():
				if !p.IsRelayed || p.Via == nil || p.Via.ID != exit.ID.String() {
					t.Fatalf("plain gateway with alternate exit = %+v, want via RelayedBy", p)
				}
			}
		}

		// Auto-relay still applies under bypass — traffic is not forced direct.
		exitClient.AutoRelayedPeers = map[string]string{backup.ID.String(): gw.ID.String()}
		stubMetrics(t, map[string]map[string]models.Metric{
			node.ID.String(): {
				exit.ID.String():    {Connected: true, Latency: 30, PercentUp: 97},
				primary.ID.String(): {Connected: true, Latency: 15, PercentUp: 99},
				backup.ID.String():  {Connected: true, Latency: 12, PercentUp: 99.5},
				gw.ID.String():      {Connected: true, Latency: 8, PercentUp: 100},
			},
			gw.ID.String(): {
				backup.ID.String(): {Connected: true, Latency: 6, PercentUp: 90},
			},
		})
		// Prefer backup via metric so the auto-relayed router is the one in use.
		eliAuto := append([]schema.Egress(nil), bypassing...)
		eliAuto[0].Nodes = datatypes.JSONMap{backup.ID.String(): json.Number("10"), primary.ID.String(): json.Number("20")}
		b = newTestStatusBuilder(eliAuto, NetworkStatusOptions{IncludeEgress: true, IncludePeers: true}, &exitClient, primary, backup, exit, gw)
		got = b.nodeNetworkStatus(&exitClient)
		e = got.Egresses[0]
		if e.RoutingNode.ID != backup.ID.String() || !e.IsRelayed || e.Via == nil || e.Via.ID != gw.ID.String() ||
			e.LatencyMs != 14 || e.PercentUp != 90 {
			t.Fatalf("with bypass+auto-relay: path = %+v, want backup via auto-relay at 14ms", e)
		}
		for _, p := range got.Peers {
			switch p.PeerID {
			case backup.ID.String():
				if !p.IsRelayed || p.Via == nil || p.Via.ID != gw.ID.String() {
					t.Fatalf("with bypass+auto-relay: backup peer = %+v, want via gw", p)
				}
			case primary.ID.String(), exit.ID.String():
				if p.IsRelayed {
					t.Fatalf("with bypass+auto-relay: peer %s = %+v, want direct", p.PeerID, p)
				}
			}
		}
	})

	t.Run("auto-relayed router without exit", func(t *testing.T) {
		autoRelayed := *node
		autoRelayed.AutoRelayedPeers = map[string]string{backup.ID.String(): gw.ID.String()}
		stubMetrics(t, map[string]map[string]models.Metric{
			node.ID.String(): {
				gw.ID.String():      {Connected: true, Latency: 8, PercentUp: 100},
				primary.ID.String(): {Connected: false, Latency: 999},
				backup.ID.String():  {Connected: true, Latency: 12},
			},
			gw.ID.String(): {backup.ID.String(): {Connected: true, Latency: 6, PercentUp: 90}},
		})
		b := newTestStatusBuilder(eli, NetworkStatusOptions{IncludeEgress: true}, &autoRelayed, primary, backup, gw)
		e := b.nodeNetworkStatus(&autoRelayed).Egresses[0]
		if e.RoutingNode.ID != backup.ID.String() || e.RoutingNodesConnected != 1 || (e.Via == nil || e.Via.ID != gw.ID.String()) ||
			!e.Connected || e.LatencyMs != 14 || e.PercentUp != 90 {
			t.Fatalf("path = %+v, want backup via auto-relay at 14ms", e)
		}
	})

	t.Run("selected internet egress", func(t *testing.T) {
		exitClient := *node
		exitClient.SelectedInternetEgressID = "inet"
		// An active exit relays the node through the exit routing node (IGW client).
		exitClient.IsRelayed, exitClient.RelayedBy, exitClient.InternetGwID = true, exit.ID.String(), exit.ID.String()
		stubMetrics(t, map[string]map[string]models.Metric{node.ID.String(): {
			exit.ID.String(): {Connected: true, Latency: 30, PercentUp: 98},
		}})

		b := newTestStatusBuilder(eli, NetworkStatusOptions{IncludeEgress: true}, &exitClient, primary, backup, exit)
		got := b.nodeNetworkStatus(&exitClient)
		if got.TotalEgresses != 2 || len(got.Egresses) != 2 {
			t.Fatalf("egresses = %v, want lan and inet", got.Egresses)
		}
		e := got.Egresses[1]
		if e.EgressID != "inet" || !e.IsInternet || len(e.Ranges) != 1 || e.Ranges[0] != IPv4Network {
			t.Fatalf("internet egress identity = %+v", e)
		}
		if e.RoutingNode.ID != exit.ID.String() || !e.Connected || e.LatencyMs != 30 || e.IsRelayed {
			t.Fatalf("internet egress link = %+v, want direct to exit at 30ms", e)
		}

		// IPv6 default route only when the exit host has a public IPv6 endpoint.
		b.hosts[exit.HostID.String()] = schema.Host{EndpointIPv6: net.ParseIP("2001:db8::1")}
		if e := b.nodeNetworkStatus(&exitClient).Egresses[1]; len(e.Ranges) != 2 || e.Ranges[1] != IPv6Network {
			t.Fatalf("internet egress ranges = %v, want IPv4 and IPv6 defaults", e.Ranges)
		}
	})

	t.Run("failed-open exit does not route all traffic", func(t *testing.T) {
		// Exit down: the sticky selection stays, the IGW relay is cleared.
		failedOpen := *node
		failedOpen.SelectedInternetEgressID = "inet"
		b := newTestStatusBuilder(eli, NetworkStatusOptions{IncludeEgress: true}, &failedOpen, primary, backup, exit)
		got := b.nodeNetworkStatus(&failedOpen)
		if got.InternetGatewayNodeID != "" || got.TotalEgresses != 1 || got.Egresses[0].EgressID != "lan" {
			t.Fatalf("internet gateway = %q, egresses = %v; want none and only lan", got.InternetGatewayNodeID, got.Egresses)
		}
	})

	t.Run("exit denied by policy does not route all traffic", func(t *testing.T) {
		exitClient := *node
		exitClient.SelectedInternetEgressID = "inet"
		exitClient.IsRelayed, exitClient.RelayedBy, exitClient.InternetGwID = true, exit.ID.String(), exit.ID.String()
		b := newTestStatusBuilder(eli, NetworkStatusOptions{IncludeEgress: true}, &exitClient, primary, backup, exit)
		b.deviceDefault = false
		if got := b.nodeNetworkStatus(&exitClient); got.InternetGatewayNodeID != "" || got.TotalEgresses != 0 {
			t.Fatalf("internet gateway = %q, egresses = %d; want none", got.InternetGatewayNodeID, got.TotalEgresses)
		}
	})

	t.Run("counts without details", func(t *testing.T) {
		b := newTestStatusBuilder(eli, NetworkStatusOptions{}, node, primary, backup)
		got := b.nodeNetworkStatus(node)
		if got.TotalEgresses != 1 || got.Egresses != nil || got.TotalPeers != 2 || got.Peers != nil {
			t.Fatalf("got egresses %d %v, peers %d %v", got.TotalEgresses, got.Egresses, got.TotalPeers, got.Peers)
		}
	})
}

func TestExtClientNetworkStatusEgresses(t *testing.T) {
	gw, router := newTestNode(), newTestNode()
	ext := &models.Node{IsStatic: true, StaticNode: models.ExtClient{ClientID: "laptop", IngressGatewayID: gw.ID.String(), Enabled: true}}
	eli := []schema.Egress{
		{ID: "via-gw", Status: true, Range: "10.0.0.0/24", Nodes: datatypes.JSONMap{gw.ID.String(): json.Number("10")}},
		{ID: "via-router", Status: true, Range: "10.1.0.0/24", Nodes: datatypes.JSONMap{router.ID.String(): json.Number("10")}},
	}
	stubMetrics(t, map[string]map[string]models.Metric{gw.ID.String(): {
		"laptop":           {Connected: true, Latency: 20, PercentUp: 100, TotalSent: 5, TotalReceived: 7},
		router.ID.String(): {Connected: true, Latency: 3, PercentUp: 90},
	}})
	b := newTestStatusBuilder(eli, NetworkStatusOptions{IncludePeers: true, IncludeEgress: true}, gw, router)

	got := b.extClientNetworkStatus(ext)
	if got.Kind != models.NetworkNodeKindExtClient || got.ConnectedPeers != 1 || len(got.Peers) != 1 {
		t.Fatalf("got %+v", got)
	}
	if p := got.Peers[0]; p.BytesSent != 7 || p.BytesReceived != 5 {
		t.Fatalf("peer bytes = %d/%d, want flipped 7/5", p.BytesSent, p.BytesReceived)
	}
	if got.TotalEgresses != 2 || got.ConnectedEgresses != 2 {
		t.Fatalf("egress counts = %d/%d, want 2/2", got.ConnectedEgresses, got.TotalEgresses)
	}
	byID := map[string]models.NetworkEgressStatus{}
	for _, e := range got.Egresses {
		byID[e.EgressID] = e
	}
	if e := byID["via-gw"]; e.IsRelayed || e.LatencyMs != 20 || e.RoutingNode.ID != gw.ID.String() {
		t.Fatalf("via-gw = %+v, want direct at 20ms", e)
	}
	if e := byID["via-router"]; !e.IsRelayed || (e.Via == nil || e.Via.ID != gw.ID.String()) ||
		e.LatencyMs != 23 || e.PercentUp != 90 {
		t.Fatalf("via-router = %+v, want relayed via gateway at 23ms", e)
	}

	// An extclient routes all traffic only through an exit its own gateway routes.
	withExit := append(eli,
		schema.Egress{ID: "inet-gw", Status: true, Type: schema.EgressTypeInternet, Nodes: datatypes.JSONMap{gw.ID.String(): json.Number("1")}},
		schema.Egress{ID: "inet-other", Status: true, Type: schema.EgressTypeInternet, Nodes: datatypes.JSONMap{router.ID.String(): json.Number("1")}},
	)
	b = newTestStatusBuilder(withExit, NetworkStatusOptions{IncludeEgress: true}, gw, router)
	exitExt := *ext
	exitExt.SelectedInternetEgressID = "inet-gw"
	got = b.extClientNetworkStatus(&exitExt)
	if got.InternetGatewayNodeID != gw.ID.String() || got.TotalEgresses != 3 {
		t.Fatalf("internet gateway = %s, egresses = %d; want %s, 3", got.InternetGatewayNodeID, got.TotalEgresses, gw.ID)
	}
	if e := got.Egresses[2]; e.EgressID != "inet-gw" || !e.IsInternet || e.RoutingNode.ID != gw.ID.String() ||
		e.IsRelayed || e.LatencyMs != 20 {
		t.Fatalf("internet egress = %+v, want direct to gateway at 20ms", e)
	}

	exitExt.SelectedInternetEgressID = "inet-other"
	if got := b.extClientNetworkStatus(&exitExt); got.InternetGatewayNodeID != "" || got.TotalEgresses != 2 {
		t.Fatalf("exit on another node: internet gateway = %q, egresses = %d; want none, 2", got.InternetGatewayNodeID, got.TotalEgresses)
	}
}

func TestPickEgressRouter(t *testing.T) {
	routers := []egressRouter{{"a", 1}, {"b", 2}, {"c", 3}}
	connected := map[string]bool{"b": true, "c": true}
	if id, n := pickEgressRouter(routers, "", func(id string) bool { return connected[id] }); id != "b" || n != 2 {
		t.Fatalf("pickEgressRouter() = %s, %d; want b, 2", id, n)
	}
	if id, n := pickEgressRouter(routers, "", func(string) bool { return false }); id != "a" || n != 0 {
		t.Fatalf("pickEgressRouter() with none connected = %s, %d; want a, 0", id, n)
	}
	if id, n := pickEgressRouter(routers, "c", func(id string) bool { return connected[id] }); id != "c" || n != 2 {
		t.Fatalf("pickEgressRouter() with preferred router = %s, %d; want c, 2", id, n)
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
