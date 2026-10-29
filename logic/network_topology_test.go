package logic

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func TestNetworkTopologyRoles(t *testing.T) {
	gw := newTestNode()
	gw.HostID = uuid.New()
	gw.IsGw = true
	gw.IsIngressGateway = true
	gw.IsRelay = true
	gw.IsAutoRelay = true
	gw.Status = schema.OnlineSt
	gw.Connected = true

	site := newTestNode()
	site.HostID = uuid.New()
	site.Status = schema.OnlineSt

	tagged := newTestNode()
	tagged.Tags = map[models.TagID]struct{}{"routers": {}}
	tagged.Status = schema.OnlineSt

	plain := newTestNode()
	plain.Status = schema.OfflineSt

	// An internet-gateway flag without an internet egress is not an exit node.
	flagOnly := newTestNode()
	flagOnly.IsInternetGateway = true

	userA := newTestNode()
	userA.HostID = uuid.New()
	userA.OwnerID = "alice"
	userA.Status = schema.OnlineSt
	userA.Connected = true
	userB := newTestNode()
	userB.OwnerID = "alice"

	rac := &models.Node{IsStatic: true, IsUserNode: true}
	rac.StaticNode.ClientID = "rac-alice"
	rac.StaticNode.OwnerID = "alice"
	rac.StaticNode.Enabled = true
	rac.StaticNode.Location = "40.7,-74.0"
	rac.StaticNode.Country = "US"

	ext := &models.Node{IsStatic: true}
	ext.StaticNode.ClientID = "site-client"
	ext.StaticNode.IngressGatewayID = gw.ID.String()
	ext.StaticNode.Enabled = true
	ext.StaticNode.Location = "51.5,-0.1"
	ext.StaticNode.Country = "GB"

	eli := []schema.Egress{
		{ID: "exit", Name: "exit", Status: true, Type: schema.EgressTypeInternet, Range: "*", Nodes: datatypes.JSONMap{gw.ID.String(): float64(1)}},
		{ID: "lan", Name: "office", Status: true, Type: schema.EgressTypeCIDR, Range: "10.0.0.0/24", Nodes: datatypes.JSONMap{site.ID.String(): float64(10)}},
		{ID: "crm", Name: "crm", Status: true, Type: schema.EgressTypeDomain, Tags: datatypes.JSONMap{"routers": float64(1)}},
		{ID: "old-exit", Name: "old", Status: false, Type: schema.EgressTypeInternet, Range: "*", Nodes: datatypes.JSONMap{flagOnly.ID.String(): float64(1)}},
	}
	b := newTestStatusBuilder(eli, NetworkStatusOptions{}, gw, site, tagged, plain, flagOnly, userA, userB)
	b.hosts[gw.HostID.String()] = schema.Host{Name: "gw", Location: "48.8,2.3", CountryCode: "FR"}
	b.hosts[site.HostID.String()] = schema.Host{Name: "site", Location: "52.3,4.9", CountryCode: "NL"}
	b.hosts[userA.HostID.String()] = schema.Host{Name: "laptop", Location: "37.7,-122.4", CountryCode: "US", OwnerUsername: "alice"}
	b.names[rac.StaticNode.ClientID] = rac.StaticNode.ClientID
	b.names[ext.StaticNode.ClientID] = ext.StaticNode.ClientID

	topo := b.networkTopology("net", []models.Node{*gw, *site, *tagged, *plain, *flagOnly, *userA, *userB, *rac, *ext})

	if topo.Network != "net" || len(topo.Users) != 1 || topo.Users[0].Username != "alice" || len(topo.Users[0].Devices) != 3 {
		t.Fatalf("users = %+v", topo.Users)
	}
	deviceIDs := map[string]models.NetworkTopologyNode{}
	for _, d := range topo.Users[0].Devices {
		deviceIDs[d.ID] = d
		if d.Kind != models.NetworkNodeKindUser || d.OwnerID != "alice" || d.IsGw || len(d.Egresses) != 0 {
			t.Fatalf("user device %+v", d)
		}
	}
	if _, ok := deviceIDs[userA.ID.String()]; !ok {
		t.Fatal("missing alice device")
	}
	if _, ok := deviceIDs[rac.StaticNode.ClientID]; !ok {
		t.Fatal("missing RAC device")
	}
	if deviceIDs[userA.ID.String()].Status != schema.OnlineSt || !deviceIDs[userA.ID.String()].Connected {
		t.Fatalf("user status = %+v", deviceIDs[userA.ID.String()])
	}
	if deviceIDs[userA.ID.String()].Location != "37.7,-122.4" || deviceIDs[userA.ID.String()].Country != "US" {
		t.Fatalf("user location = %+v", deviceIDs[userA.ID.String()])
	}
	if deviceIDs[rac.StaticNode.ClientID].Location != "40.7,-74.0" || deviceIDs[rac.StaticNode.ClientID].Country != "US" {
		t.Fatalf("rac location = %+v", deviceIDs[rac.StaticNode.ClientID])
	}

	servers := map[string]models.NetworkTopologyNode{}
	for _, s := range topo.Servers {
		servers[s.ID] = s
		if s.Kind != models.NetworkNodeKindNode {
			t.Fatalf("server kind %s", s.Kind)
		}
	}
	if len(servers) != 5 {
		t.Fatalf("servers = %+v", topo.Servers)
	}
	gotGW := servers[gw.ID.String()]
	if !gotGW.IsGw {
		t.Fatalf("gateway is_gw = false")
	}
	if gotGW.Location != "48.8,2.3" || gotGW.Country != "FR" {
		t.Fatalf("gateway location = %+v", gotGW)
	}
	if len(gotGW.Egresses) != 1 || gotGW.Egresses[0].Type != string(schema.EgressTypeInternet) || gotGW.Egresses[0].Role != models.NetworkEgressRoleInternetGateway {
		t.Fatalf("gateway egresses = %+v", gotGW.Egresses)
	}
	gotSite := servers[site.ID.String()]
	if gotSite.IsGw || len(gotSite.Egresses) != 1 || gotSite.Egresses[0].Role != models.NetworkEgressRoleEgress || gotSite.Egresses[0].Type != string(schema.EgressTypeCIDR) {
		t.Fatalf("site = %+v", gotSite)
	}
	if gotSite.Location != "52.3,4.9" || gotSite.Country != "NL" {
		t.Fatalf("site location = %+v", gotSite)
	}
	gotTagged := servers[tagged.ID.String()]
	if len(gotTagged.Egresses) != 1 || gotTagged.Egresses[0].Type != string(schema.EgressTypeDomain) || gotTagged.Egresses[0].Role != models.NetworkEgressRoleEgress {
		t.Fatalf("tagged = %+v", gotTagged)
	}
	if servers[plain.ID.String()].IsGw || len(servers[plain.ID.String()].Egresses) != 0 {
		t.Fatalf("plain = %+v", servers[plain.ID.String()])
	}
	if servers[flagOnly.ID.String()].IsGw || len(servers[flagOnly.ID.String()].Egresses) != 0 {
		t.Fatalf("flag-only internet gateway = %+v", servers[flagOnly.ID.String()])
	}
	if len(topo.ExtClients) != 1 || topo.ExtClients[0].ID != ext.StaticNode.ClientID || topo.ExtClients[0].GatewayNodeID != gw.ID.String() {
		t.Fatalf("extclients = %+v", topo.ExtClients)
	}
	if topo.ExtClients[0].Location != "51.5,-0.1" || topo.ExtClients[0].Country != "GB" {
		t.Fatalf("extclient location = %+v", topo.ExtClients[0])
	}
}

func TestNodeConnectionsUserServerAndACL(t *testing.T) {
	alice := newTestNode()
	alice.OwnerID = "alice"
	bob := newTestNode()
	bob.OwnerID = "bob"
	allowed := newTestNode()
	denied := newTestNode()

	b := newTestStatusBuilder(nil, NetworkStatusOptions{}, alice, bob, allowed, denied)
	b.deviceDefault = false
	origUser := IsUserAllowedToCommunicate
	origPeer := IsPeerAllowed
	IsUserAllowedToCommunicate = func(_ context.Context, _ string, peer models.Node) (bool, []models.Acl) {
		return peer.ID == allowed.ID, nil
	}
	IsPeerAllowed = func(_ context.Context, _, _ models.Node, _ bool) bool { return false }
	t.Cleanup(func() {
		IsUserAllowedToCommunicate = origUser
		IsPeerAllowed = origPeer
	})

	nodes := []models.Node{*alice, *bob, *allowed, *denied}
	aliceConns, err := b.nodeConnections(nodes, alice.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(aliceConns.Connections) != 1 || aliceConns.Connections[0].PeerID != allowed.ID.String() || aliceConns.Connections[0].Type != models.NetworkConnectionP2P {
		t.Fatalf("alice connections = %+v", aliceConns.Connections)
	}

	serverConns, err := b.nodeConnections(nodes, allowed.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(serverConns.Connections) != 2 {
		t.Fatalf("allowed server connections = %+v", serverConns.Connections)
	}
	seen := map[string]models.NetworkNodeKind{}
	for _, c := range serverConns.Connections {
		seen[c.PeerID] = c.Kind
	}
	if seen[alice.ID.String()] != models.NetworkNodeKindUser || seen[bob.ID.String()] != models.NetworkNodeKindUser {
		t.Fatalf("server peers = %+v", seen)
	}
	if _, ok := seen[denied.ID.String()]; ok {
		t.Fatal("device policy denial still produced a server line")
	}

	deniedConns, err := b.nodeConnections(nodes, denied.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(deniedConns.Connections) != 0 {
		t.Fatalf("denied server connections = %+v", deniedConns.Connections)
	}
	if _, err := b.nodeConnections(nodes, uuid.NewString()); err != ErrTopologyNodeNotFound {
		t.Fatalf("missing node err = %v", err)
	}
}

func TestNodeConnectionsRACExtClientUserPolicy(t *testing.T) {
	allowed := newTestNode()
	denied := newTestNode()
	rac := &models.Node{IsStatic: true, IsUserNode: true}
	rac.StaticNode.ClientID = "rac-alice"
	rac.StaticNode.OwnerID = "alice"
	rac.StaticNode.Enabled = true

	b := newTestStatusBuilder(nil, NetworkStatusOptions{}, allowed, denied)
	b.names[rac.StaticNode.ClientID] = rac.StaticNode.ClientID
	origUser := IsUserAllowedToCommunicate
	IsUserAllowedToCommunicate = func(_ context.Context, userName string, peer models.Node) (bool, []models.Acl) {
		return userName == "alice" && peer.ID == allowed.ID, nil
	}
	t.Cleanup(func() { IsUserAllowedToCommunicate = origUser })

	nodes := []models.Node{*allowed, *denied, *rac}
	racConns, err := b.nodeConnections(nodes, rac.StaticNode.ClientID)
	if err != nil {
		t.Fatal(err)
	}
	if len(racConns.Connections) != 1 || racConns.Connections[0].PeerID != allowed.ID.String() {
		t.Fatalf("rac connections = %+v", racConns.Connections)
	}

	deniedConns, err := b.nodeConnections(nodes, denied.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range deniedConns.Connections {
		if c.PeerID == rac.StaticNode.ClientID {
			t.Fatalf("denying user policy still produced a line to the RAC: %+v", deniedConns.Connections)
		}
	}
	if len(deniedConns.Connections) != 1 || deniedConns.Connections[0].PeerID != allowed.ID.String() {
		t.Fatalf("denied server connections = %+v", deniedConns.Connections)
	}

	allowedConns, err := b.nodeConnections(nodes, allowed.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]struct{}{}
	for _, c := range allowedConns.Connections {
		seen[c.PeerID] = struct{}{}
	}
	if _, ok := seen[rac.StaticNode.ClientID]; !ok {
		t.Fatalf("allowed server missing RAC: %+v", allowedConns.Connections)
	}
	if _, ok := seen[denied.ID.String()]; !ok {
		t.Fatalf("device default should still link servers: %+v", allowedConns.Connections)
	}
}

func TestNodeConnectionsMemoizesUserPolicyLookups(t *testing.T) {
	alice := newTestNode()
	alice.OwnerID = "alice"
	allowed := newTestNode()
	other := newTestNode()
	third := newTestNode()

	b := newTestStatusBuilder(nil, NetworkStatusOptions{}, alice, allowed, other, third)
	var calls int
	origUser := IsUserAllowedToCommunicate
	IsUserAllowedToCommunicate = func(_ context.Context, userName string, peer models.Node) (bool, []models.Acl) {
		calls++
		return userName == "alice" && peer.ID == allowed.ID, nil
	}
	t.Cleanup(func() { IsUserAllowedToCommunicate = origUser })

	nodes := []models.Node{*alice, *allowed, *other, *third}
	got, err := b.nodeConnections(nodes, alice.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Connections) != 1 || got.Connections[0].PeerID != allowed.ID.String() {
		t.Fatalf("alice connections = %+v", got.Connections)
	}
	if calls != 3 {
		t.Fatalf("user policy lookups = %d, want one per server", calls)
	}

	calls = 0
	if _, err := b.nodeConnections(nodes, alice.ID.String()); err != nil {
		t.Fatal(err)
	}
	fromAllowed, err := b.nodeConnections(nodes, allowed.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("repeat user policy lookups = %d, want cached results", calls)
	}
	seen := map[string]struct{}{}
	for _, c := range fromAllowed.Connections {
		seen[c.PeerID] = struct{}{}
	}
	if _, ok := seen[alice.ID.String()]; !ok {
		t.Fatalf("allowed server missing alice: %+v", fromAllowed.Connections)
	}
}

func TestNodeConnectionsUsesLoadedUserPolicy(t *testing.T) {
	allowed := newTestNode()
	denied := newTestNode()
	site := newTestNode()
	relay := newTestNode()
	relay.IsRelay = true
	relay.RelayedNodes = []string{allowed.ID.String()}
	alice := newTestNode()
	alice.OwnerID = "alice"
	rac := &models.Node{IsStatic: true, IsUserNode: true}
	rac.StaticNode.ClientID = "rac-alice"
	rac.StaticNode.OwnerID = "alice"
	rac.StaticNode.Enabled = true

	eli := []schema.Egress{{
		ID:     "lan",
		Status: true,
		Nodes:  datatypes.JSONMap{site.ID.String(): float64(1)},
	}}
	b := newTestStatusBuilder(eli, NetworkStatusOptions{}, alice, allowed, denied, site, relay)
	b.names[rac.StaticNode.ClientID] = rac.StaticNode.ClientID
	b.network = "net"
	b.userPoliciesLoaded = true
	b.userDefault = false
	b.users["alice"] = &schema.User{Username: "alice"}
	b.userPolicies = []models.Acl{{
		Enabled:  true,
		RuleType: models.UserPolicy,
		Src:      []schema.AclPolicyTag{{ID: models.UserAclID, Value: "alice"}},
		Dst: []schema.AclPolicyTag{
			{ID: models.NodeID, Value: allowed.ID.String()},
			{ID: models.EgressID, Value: "lan"},
		},
	}}

	origUser := IsUserAllowedToCommunicate
	origGet := getNodeByID
	IsUserAllowedToCommunicate = func(context.Context, string, models.Node) (bool, []models.Acl) {
		t.Fatal("loaded user policy was queried again")
		return false, nil
	}
	getNodeByID = func(id string) (models.Node, error) {
		t.Fatalf("relayed node %s was loaded again", id)
		return models.Node{}, ErrTopologyNodeNotFound
	}
	t.Cleanup(func() {
		IsUserAllowedToCommunicate = origUser
		getNodeByID = origGet
	})

	nodes := []models.Node{*alice, *allowed, *denied, *site, *relay, *rac}
	aliceConns, err := b.nodeConnections(nodes, alice.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	alicePeers := indexConns(t, aliceConns.Connections)
	if _, ok := alicePeers[allowed.ID.String()]; !ok {
		t.Fatalf("alice missing allowed server: %+v", aliceConns.Connections)
	}
	if _, ok := alicePeers[site.ID.String()]; !ok {
		t.Fatalf("alice missing egress router: %+v", aliceConns.Connections)
	}
	if _, ok := alicePeers[relay.ID.String()]; !ok {
		t.Fatalf("alice missing relay of an allowed node: %+v", aliceConns.Connections)
	}
	if _, ok := alicePeers[denied.ID.String()]; ok {
		t.Fatalf("denying user policy still linked alice to denied: %+v", aliceConns.Connections)
	}

	racConns, err := b.nodeConnections(nodes, rac.StaticNode.ClientID)
	if err != nil {
		t.Fatal(err)
	}
	racPeers := indexConns(t, racConns.Connections)
	if _, ok := racPeers[allowed.ID.String()]; !ok {
		t.Fatalf("rac missing allowed server: %+v", racConns.Connections)
	}
	if _, ok := racPeers[site.ID.String()]; !ok {
		t.Fatalf("rac missing egress router: %+v", racConns.Connections)
	}
	if _, ok := racPeers[denied.ID.String()]; ok {
		t.Fatalf("denying user policy still linked rac to denied: %+v", racConns.Connections)
	}

	deniedConns, err := b.nodeConnections(nodes, denied.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range deniedConns.Connections {
		if c.PeerID == alice.ID.String() || c.PeerID == rac.StaticNode.ClientID {
			t.Fatalf("denied server linked to a user the policy excludes: %+v", deniedConns.Connections)
		}
	}
}

func TestNodeConnectionsRelayedExitAndExtClient(t *testing.T) {
	relay := newTestNode()
	relay.IsRelay = true
	exitNode := newTestNode()
	site := newTestNode()
	plain := newTestNode()
	client := newTestNode()
	client.IsRelayed = true
	client.RelayedBy = relay.ID.String()
	exitClient := newTestNode()
	exitClient.IsRelayed = true
	exitClient.RelayedBy = exitNode.ID.String()
	exitClient.SelectedInternetEgressID = "exit"
	exitClient.InternetGwID = exitNode.ID.String()
	// Peer manually RelayedBy a node that is also an internet-egress router —
	// path type must stay "relayed", not flip to "exit".
	relayedByExit := newTestNode()
	relayedByExit.IsRelayed = true
	relayedByExit.RelayedBy = exitNode.ID.String()

	eli := []schema.Egress{
		{ID: "exit", Name: "exit", Status: true, Type: schema.EgressTypeInternet, Range: "*", BypassEgressRoutes: true, Nodes: datatypes.JSONMap{exitNode.ID.String(): float64(1)}},
		{ID: "lan", Name: "office", Status: true, Type: schema.EgressTypeCIDR, Range: "10.1.0.0/16", Nodes: datatypes.JSONMap{site.ID.String(): float64(10)}},
	}
	b := newTestStatusBuilder(eli, NetworkStatusOptions{}, relay, exitNode, site, plain, client, exitClient, relayedByExit)
	b.names[relay.ID.String()] = "fra-gw"
	b.names[exitNode.ID.String()] = "exit"
	b.names[relayedByExit.ID.String()] = "ubuntu-fra"

	stubMetrics(t, map[string]map[string]models.Metric{
		client.ID.String(): {
			relay.ID.String(): {Connected: true, Latency: 10, PercentUp: 99},
		},
		relay.ID.String(): {
			plain.ID.String(): {Connected: true, Latency: 15, PercentUp: 90},
		},
		plain.ID.String(): {
			exitNode.ID.String(): {Connected: true, Latency: 5, PercentUp: 99},
		},
		exitNode.ID.String(): {
			relayedByExit.ID.String(): {Connected: true, Latency: 8, PercentUp: 95},
		},
	})

	nodes := []models.Node{*relay, *exitNode, *site, *plain, *client, *exitClient, *relayedByExit}
	got, err := b.nodeConnections(nodes, client.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	byPeer := indexConns(t, got.Connections)
	if c := byPeer[relay.ID.String()]; c.Type != models.NetworkConnectionP2P || c.Via != nil || !c.Connected || c.LatencyMs != 10 {
		t.Fatalf("link to relay = %+v", c)
	}
	if c := byPeer[plain.ID.String()]; c.Type != models.NetworkConnectionRelayed || c.Via == nil || c.Via.ID != relay.ID.String() || c.Via.Name != "fra-gw" || !c.Connected || c.LatencyMs != 25 || c.PercentUp != 90 {
		t.Fatalf("relayed link = %+v", c)
	}

	exitConns, err := b.nodeConnections(nodes, exitClient.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	exitByPeer := indexConns(t, exitConns.Connections)
	if c := exitByPeer[exitNode.ID.String()]; c.Type != models.NetworkConnectionP2P || c.Via != nil {
		t.Fatalf("link to exit = %+v", c)
	}
	if c := exitByPeer[site.ID.String()]; c.Type != models.NetworkConnectionP2P || c.Via != nil {
		t.Fatalf("bypass site egress = %+v", c)
	}
	if c := exitByPeer[plain.ID.String()]; c.Type != models.NetworkConnectionRelayed || c.Via == nil || c.Via.ID != exitNode.ID.String() || c.Via.Name != "exit" || c.Connected {
		t.Fatalf("exit-client peer via RelayedBy exit router = %+v, want relayed", c)
	}

	plainConns, err := b.nodeConnections(nodes, plain.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	plainByPeer := indexConns(t, plainConns.Connections)
	if c := plainByPeer[relayedByExit.ID.String()]; c.Type != models.NetworkConnectionRelayed || c.Via == nil || c.Via.ID != exitNode.ID.String() || !c.Connected || c.LatencyMs != 13 || c.PercentUp != 95 {
		t.Fatalf("peer RelayedBy exit router = %+v, want relayed via exit", c)
	}

	ext := &models.Node{IsStatic: true}
	ext.StaticNode.ClientID = "client-1"
	ext.StaticNode.IngressGatewayID = relay.ID.String()
	b.names[ext.StaticNode.ClientID] = ext.StaticNode.ClientID
	withExt := append(nodes, *ext)
	extConns, err := b.nodeConnections(withExt, ext.StaticNode.ClientID)
	if err != nil {
		t.Fatal(err)
	}
	extByPeer := indexConns(t, extConns.Connections)
	if c := extByPeer[relay.ID.String()]; c.Type != models.NetworkConnectionP2P || c.Kind != models.NetworkNodeKindNode {
		t.Fatalf("extclient to ingress = %+v", c)
	}
	if c := extByPeer[plain.ID.String()]; c.Type != models.NetworkConnectionRelayed || c.Via == nil || c.Via.ID != relay.ID.String() {
		t.Fatalf("extclient relayed = %+v", c)
	}

	fromRelay, err := b.nodeConnections(withExt, relay.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	relayByPeer := indexConns(t, fromRelay.Connections)
	if c := relayByPeer[ext.StaticNode.ClientID]; c.Type != models.NetworkConnectionP2P || c.Kind != models.NetworkNodeKindExtClient {
		t.Fatalf("ingress to extclient = %+v", c)
	}
}

func TestMeshConnectionPathAutoRelayedViaExit(t *testing.T) {
	exitNode := newTestNode()
	peer := newTestNode()
	viewer := newTestNode()
	viewer.SelectedInternetEgressID = "exit"
	viewer.InternetGwID = exitNode.ID.String()
	viewer.AutoRelayedPeers = map[string]string{peer.ID.String(): exitNode.ID.String()}

	eli := []schema.Egress{
		{ID: "exit", Name: "exit", Status: true, Type: schema.EgressTypeInternet, Range: "*", BypassEgressRoutes: true, Nodes: datatypes.JSONMap{exitNode.ID.String(): float64(1)}},
	}
	b := newTestStatusBuilder(eli, NetworkStatusOptions{}, viewer, exitNode, peer)

	tests := []struct {
		name     string
		node     *models.Node
		peer     *models.Node
		wantType models.NetworkConnectionType
		wantVia  string
	}{
		{
			name:     "auto-relayed through exit router stays relayed",
			node:     viewer,
			peer:     peer,
			wantType: models.NetworkConnectionRelayed,
			wantVia:  exitNode.ID.String(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotVia := b.meshConnectionPath(tt.node, tt.peer)
			if gotType != tt.wantType || gotVia != tt.wantVia {
				t.Fatalf("meshConnectionPath = (%q, %q), want (%q, %q)", gotType, gotVia, tt.wantType, tt.wantVia)
			}
		})
	}
}

func TestNodeConnectionsExtClientLiveness(t *testing.T) {
	gw := newTestNode()
	gw.IsIngressGateway = true
	gw.IsRelay = true
	viewer := newTestNode()
	relayed := newTestNode()
	relayed.IsRelayed = true
	relayed.RelayedBy = gw.ID.String()
	partial := newTestNode()
	down := newTestNode()

	ext := &models.Node{IsStatic: true}
	ext.StaticNode.ClientID = "laptop"
	ext.StaticNode.IngressGatewayID = gw.ID.String()
	ext.StaticNode.Enabled = true

	b := newTestStatusBuilder(nil, NetworkStatusOptions{}, gw, viewer, relayed, partial, down)
	b.names[gw.ID.String()] = "gateway"
	b.names[ext.StaticNode.ClientID] = "laptop"
	stubMetrics(t, map[string]map[string]models.Metric{
		viewer.ID.String(): {
			gw.ID.String(): {Connected: true, Latency: 5, PercentUp: 80},
		},
		relayed.ID.String(): {
			gw.ID.String(): {Connected: true, Latency: 4, PercentUp: 90},
		},
		down.ID.String(): {
			gw.ID.String(): {Connected: false, Latency: 3, PercentUp: 10},
		},
		gw.ID.String(): {
			ext.StaticNode.ClientID: {Connected: true, Latency: 7, PercentUp: 70},
		},
	})

	nodes := []models.Node{*gw, *viewer, *relayed, *partial, *down, *ext}
	fromViewer, err := b.nodeConnections(nodes, viewer.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if c := indexConns(t, fromViewer.Connections)[ext.StaticNode.ClientID]; c.Type != models.NetworkConnectionRelayed || c.Via == nil || c.Via.ID != gw.ID.String() || !c.Connected || c.LatencyMs != 12 || c.PercentUp != 70 {
		t.Fatalf("viewer through gateway = %+v", c)
	}

	fromRelayed, err := b.nodeConnections(nodes, relayed.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if c := indexConns(t, fromRelayed.Connections)[ext.StaticNode.ClientID]; c.Type != models.NetworkConnectionRelayed || c.Via == nil || c.Via.ID != gw.ID.String() || !c.Connected || c.LatencyMs != 11 || c.PercentUp != 70 {
		t.Fatalf("node relayed by its gateway = %+v", c)
	}

	fromGW, err := b.nodeConnections(nodes, gw.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if c := indexConns(t, fromGW.Connections)[ext.StaticNode.ClientID]; c.Type != models.NetworkConnectionP2P || c.Via != nil || !c.Connected || c.LatencyMs != 7 || c.PercentUp != 70 {
		t.Fatalf("gateway to extclient = %+v", c)
	}

	fromPartial, err := b.nodeConnections(nodes, partial.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if c := indexConns(t, fromPartial.Connections)[ext.StaticNode.ClientID]; c.Via == nil || c.Via.ID != gw.ID.String() || !c.Connected || c.LatencyMs != 7 || c.PercentUp != 70 {
		t.Fatalf("gateway hop alone = %+v", c)
	}

	fromDown, err := b.nodeConnections(nodes, down.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if c := indexConns(t, fromDown.Connections)[ext.StaticNode.ClientID]; c.Connected || c.LatencyMs != 10 || c.PercentUp != 10 {
		t.Fatalf("down first hop = %+v", c)
	}
}

func indexConns(t *testing.T, conns []models.NetworkConnection) map[string]models.NetworkConnection {
	t.Helper()
	out := make(map[string]models.NetworkConnection, len(conns))
	for _, c := range conns {
		if _, ok := out[c.PeerID]; ok {
			t.Fatalf("duplicate peer %s", c.PeerID)
		}
		out[c.PeerID] = c
	}
	return out
}

func TestNodeConnectionsLocationAndRoles(t *testing.T) {
	plain := newTestNode()
	plain.HostID = uuid.New()

	gw := newTestNode()
	gw.HostID = uuid.New()
	gw.IsGw = true
	gw.IsRelay = true

	exitNode := newTestNode()
	exitNode.HostID = uuid.New()

	site := newTestNode()
	site.HostID = uuid.New()

	userDev := newTestNode()
	userDev.OwnerID = "alice"
	userDev.HostID = uuid.New()

	eli := []schema.Egress{
		{ID: "exit", Name: "exit", Status: true, Type: schema.EgressTypeInternet, Range: "*", Nodes: datatypes.JSONMap{exitNode.ID.String(): float64(1)}},
		{ID: "lan", Name: "office", Status: true, Type: schema.EgressTypeCIDR, Range: "10.0.0.0/24", Nodes: datatypes.JSONMap{site.ID.String(): float64(10)}},
	}
	b := newTestStatusBuilder(eli, NetworkStatusOptions{}, plain, gw, exitNode, site, userDev)
	b.hosts[plain.HostID.String()] = schema.Host{Name: "plain", Location: "1.0,2.0", CountryCode: "DE"}
	b.hosts[gw.HostID.String()] = schema.Host{Name: "gw", Location: "48.8,2.3", CountryCode: "FR"}
	b.hosts[exitNode.HostID.String()] = schema.Host{Name: "exit", Location: "52.3,4.9", CountryCode: "NL"}
	b.hosts[site.HostID.String()] = schema.Host{Name: "site", Location: "51.5,-0.1", CountryCode: "GB"}
	b.hosts[userDev.HostID.String()] = schema.Host{Name: "laptop", Location: "40.7,-74.0", CountryCode: "US", OwnerUsername: "alice"}
	b.names[plain.ID.String()] = "plain"
	b.names[gw.ID.String()] = "gw"
	b.names[exitNode.ID.String()] = "exit"
	b.names[site.ID.String()] = "site"
	b.names[userDev.ID.String()] = "laptop"

	origUser := IsUserAllowedToCommunicate
	IsUserAllowedToCommunicate = func(_ context.Context, _ string, _ models.Node) (bool, []models.Acl) {
		return true, nil
	}
	t.Cleanup(func() { IsUserAllowedToCommunicate = origUser })

	nodes := []models.Node{*plain, *gw, *exitNode, *site, *userDev}
	got, err := b.nodeConnections(nodes, plain.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if got.Location != "1.0,2.0" || got.Country != "DE" || got.Name != "plain" || got.Kind != models.NetworkNodeKindNode {
		t.Fatalf("self = %+v", got)
	}
	if got.IsGw || len(got.Egresses) != 0 {
		t.Fatalf("plain meta = is_gw=%v egresses=%+v", got.IsGw, got.Egresses)
	}

	byPeer := indexConns(t, got.Connections)
	if c := byPeer[gw.ID.String()]; c.Location != "48.8,2.3" || c.Country != "FR" ||
		!c.IsGw || len(c.Egresses) != 0 {
		t.Fatalf("gateway peer = %+v", c)
	}
	if c := byPeer[exitNode.ID.String()]; c.Location != "52.3,4.9" || c.Country != "NL" ||
		c.IsGw || len(c.Egresses) != 1 ||
		c.Egresses[0].ID != "exit" || c.Egresses[0].Role != models.NetworkEgressRoleInternetGateway ||
		c.Egresses[0].Type != string(schema.EgressTypeInternet) {
		t.Fatalf("exit peer = %+v", c)
	}
	if c := byPeer[site.ID.String()]; c.Location != "51.5,-0.1" || c.Country != "GB" ||
		c.IsGw || len(c.Egresses) != 1 ||
		c.Egresses[0].ID != "lan" || c.Egresses[0].Name != "office" ||
		c.Egresses[0].Role != models.NetworkEgressRoleEgress || c.Egresses[0].Type != string(schema.EgressTypeCIDR) {
		t.Fatalf("site egress peer = %+v", c)
	}
	if c := byPeer[userDev.ID.String()]; c.Location != "40.7,-74.0" || c.Country != "US" ||
		c.IsGw || len(c.Egresses) != 0 {
		t.Fatalf("user device peer = %+v", c)
	}

	// Gateway that also routes egresses: is_gw=true, egresses list both roles.
	gwExit := newTestNode()
	gwExit.HostID = uuid.New()
	gwExit.IsGw = true
	eliBoth := []schema.Egress{
		{ID: "exit", Name: "exit", Status: true, Type: schema.EgressTypeInternet, Range: "*", Nodes: datatypes.JSONMap{gwExit.ID.String(): float64(1)}},
		{ID: "lan", Name: "office", Status: true, Type: schema.EgressTypeCIDR, Range: "10.0.0.0/24", Nodes: datatypes.JSONMap{gwExit.ID.String(): float64(10)}},
	}
	bBoth := newTestStatusBuilder(eliBoth, NetworkStatusOptions{}, plain, gwExit)
	bBoth.hosts[gwExit.HostID.String()] = schema.Host{Name: "gw-exit", Location: "0,0", CountryCode: "XX"}
	bBoth.names[gwExit.ID.String()] = "gw-exit"
	bothConns, err := bBoth.nodeConnections([]models.Node{*plain, *gwExit}, plain.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	c := indexConns(t, bothConns.Connections)[gwExit.ID.String()]
	if !c.IsGw {
		t.Fatalf("gateway+exit is_gw = false")
	}
	if len(c.Egresses) != 2 ||
		c.Egresses[0].ID != "exit" || c.Egresses[0].Role != models.NetworkEgressRoleInternetGateway ||
		c.Egresses[1].ID != "lan" || c.Egresses[1].Role != models.NetworkEgressRoleEgress {
		t.Fatalf("gateway+exit egresses = %+v", c.Egresses)
	}
}
