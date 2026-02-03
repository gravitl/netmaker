package logic

import (
	"fmt"
	netpkg "net"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// TestGetNodeStatusScale tests GetNodeStatus with varying numbers of peers and policies
func TestGetNodeStatusScale(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping scale test in short mode")
	}

	// Disable DNS mode for tests to avoid SetDNS() calls slowing down node creation
	os.Setenv("DNS_MODE", "off")
	defer os.Unsetenv("DNS_MODE")

	// Reduce logger verbosity to minimize zombie detection noise during tests
	// Zombie messages are expected when reusing the same host for multiple peers
	originalVerbosity := logger.Verbosity
	logger.Verbosity = 1 // Only show errors and above
	defer func() { logger.Verbosity = originalVerbosity }()

	// Initialize test database
	err := db.InitializeDB(schema.ListModels()...)
	require.NoError(t, err)
	defer db.CloseDB()

	err = database.InitializeDatabase()
	require.NoError(t, err)
	defer database.CloseDB()

	// Clean up any existing test networks before starting (non-blocking)
	// Run cleanup in background to avoid blocking test execution
	go cleanupAllTestNetworks(t)
	// Give cleanup a moment to start
	time.Sleep(100 * time.Millisecond)
	// Give cleanup a moment to start
	time.Sleep(100 * time.Millisecond)

	// Test with different scales
	testCases := []struct {
		name          string
		numPeers      int
		numPolicies   int
		defaultPolicy bool
		expectedMaxMs int64 // Maximum expected duration in milliseconds
	}{
		{
			name:          "Small scale: 5 peers, 5 policies",
			numPeers:      5,
			numPolicies:   5,
			defaultPolicy: false,
			expectedMaxMs: 1000,
		},
		{
			name:          "Medium scale: 50 peers, 20 policies",
			numPeers:      50,
			numPolicies:   20,
			defaultPolicy: false,
			expectedMaxMs: 3000,
		},
		{
			name:          "Large scale: 100 peers, 50 policies",
			numPeers:      100,
			numPolicies:   50,
			defaultPolicy: false,
			expectedMaxMs: 5000,
		},
		{
			name:          "Very large scale: 200 peers, 100 policies",
			numPeers:      200,
			numPolicies:   100,
			defaultPolicy: false,
			expectedMaxMs: 10000,
		},
		{
			name:          "Very large scale: 200 peers, 100 policies",
			numPeers:      500,
			numPolicies:   100,
			defaultPolicy: false,
			expectedMaxMs: 10000,
		},
		{
			name:          "Very large scale: 200 peers, 100 policies",
			numPeers:      1000,
			numPolicies:   100,
			defaultPolicy: false,
			expectedMaxMs: 10000,
		},
		{
			name:          "With default policy enabled: 100 peers, 50 policies",
			numPeers:      100,
			numPolicies:   50,
			defaultPolicy: true,
			expectedMaxMs: 2000, // Should be faster with default policy
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup test network and nodes with unique CIDR to avoid conflicts
			// Use different subnet for each test case: 10.1.x.0/24, 10.2.x.0/24, etc.
			fmt.Println("Running test case", i)
			subnet := 1 + (i % 255)
			networkID := models.NetworkID("test-network-" + uuid.New().String()[:8])
			testNode, peers, err := setupTestScenario(networkID, tc.numPeers, tc.numPolicies, subnet)
			require.NoError(t, err, "Failed to setup test scenario")
			fmt.Println("Setup test case", i)
			// Create metrics with connectivity data
			metrics := &models.Metrics{
				Connectivity: make(map[string]models.Metric),
				UpdatedAt:    time.Now(),
			}
			for _, peer := range peers {
				metrics.Connectivity[peer.ID.String()] = models.Metric{
					Connected: true,
					Latency:   10,
				}
			}
			err = logic.UpdateMetrics(testNode.ID.String(), metrics)
			require.NoError(t, err)

			// Run GetNodeStatus and measure time
			start := time.Now()
			GetNodeStatus(testNode, tc.defaultPolicy)
			duration := time.Since(start)

			t.Logf("GetNodeStatus completed in %v for %d peers and %d policies (defaultPolicy=%v)",
				duration, tc.numPeers, tc.numPolicies, tc.defaultPolicy)
			t.Logf("Average time per peer: %v", duration/time.Duration(tc.numPeers))

			// Verify status was set
			assert.NotEmpty(t, testNode.Status, "Node status should be set")

			// Performance assertion
			maxDuration := time.Duration(tc.expectedMaxMs) * time.Millisecond
			assert.Less(t, duration, maxDuration,
				"GetNodeStatus took too long: %v (expected < %v)", duration, maxDuration)

			// Cleanup
			cleanupTestScenario(networkID, testNode, peers)
		})
	}
}

// BenchmarkGetNodeStatus benchmarks GetNodeStatus with different configurations
func BenchmarkGetNodeStatus(b *testing.B) {
	// Initialize test database
	err := db.InitializeDB(schema.ListModels()...)
	if err != nil {
		b.Fatal(err)
	}
	defer db.CloseDB()

	err = database.InitializeDatabase()
	if err != nil {
		b.Fatal(err)
	}
	defer database.CloseDB()

	benchmarks := []struct {
		name          string
		numPeers      int
		numPolicies   int
		defaultPolicy bool
	}{
		{"10peers_5policies", 10, 5, false},
		{"50peers_20policies", 50, 20, false},
		{"100peers_50policies", 100, 50, false},
		{"200peers_100policies", 200, 100, false},
		{"100peers_50policies_defaultPolicy", 100, 50, true},
	}

	for i, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			// Use unique subnet for each benchmark
			subnet := 100 + (i % 155) // Use 100-254 range for benchmarks
			networkID := models.NetworkID("bench-network-" + uuid.New().String()[:8])
			testNode, peers, err := setupTestScenario(networkID, bm.numPeers, bm.numPolicies, subnet)
			if err != nil {
				b.Fatalf("Failed to setup test scenario: %v", err)
			}
			defer cleanupTestScenario(networkID, testNode, peers)

			// Create metrics
			metrics := &models.Metrics{
				Connectivity: make(map[string]models.Metric),
				UpdatedAt:    time.Now(),
			}
			for _, peer := range peers {
				metrics.Connectivity[peer.ID.String()] = models.Metric{
					Connected: true,
					Latency:   10,
				}
			}
			_ = logic.UpdateMetrics(testNode.ID.String(), metrics)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				GetNodeStatus(testNode, bm.defaultPolicy)
			}
		})
	}
}

// BenchmarkACLFiltering benchmarks the policy filtering optimization
func BenchmarkACLFiltering(b *testing.B) {
	err := db.InitializeDB(schema.ListModels()...)
	if err != nil {
		b.Fatal(err)
	}
	defer db.CloseDB()

	err = database.InitializeDatabase()
	if err != nil {
		b.Fatal(err)
	}
	defer database.CloseDB()

	networkID := models.NetworkID("filter-bench-" + uuid.New().String()[:8])
	testNode, _, err := setupTestScenario(networkID, 0, 100, 200) // 100 policies, no peers, subnet 200
	if err != nil {
		b.Fatalf("Failed to setup test scenario: %v", err)
	}

	var nodeID string
	if testNode.IsStatic {
		nodeID = testNode.StaticNode.ClientID
	} else {
		nodeID = testNode.ID.String()
	}

	var nodeTags map[string]struct{}
	if testNode.Mutex != nil {
		testNode.Mutex.Lock()
		nodeTags = make(map[string]struct{})
		for tag := range testNode.Tags {
			nodeTags[tag.String()] = struct{}{}
		}
		testNode.Mutex.Unlock()
	} else {
		nodeTags = make(map[string]struct{})
		for tag := range testNode.Tags {
			nodeTags[tag.String()] = struct{}{}
		}
	}
	nodeTags[nodeID] = struct{}{}

	allPolicies := logic.ListDevicePolicies(networkID)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Convert nodeTags to models.TagID map
		tagMap := make(map[models.TagID]struct{})
		for tagStr := range nodeTags {
			tagMap[models.TagID(tagStr)] = struct{}{}
		}
		_ = filterPoliciesForNode(allPolicies, nodeID, tagMap)
	}
}

// cleanupAllTestNetworks removes all test networks and networks with conflicting CIDRs
func cleanupAllTestNetworks(t *testing.T) {
	networks, err := logic.GetNetworks()
	if err != nil {
		return
	}

	// Delete all test networks and benchmark networks
	for _, net := range networks {
		netID := net.NetID
		isTestNetwork := (len(netID) > 12 && netID[:12] == "test-network-") ||
			(len(netID) > 14 && netID[:14] == "bench-network-") ||
			(len(netID) > 12 && netID[:12] == "filter-bench")

		// Also check if network uses test CIDR range (10.1.x.0/24 through 10.254.x.0/24)
		isTestCIDR := false
		if net.AddressRange != "" {
			_, ipNet, err := netpkg.ParseCIDR(net.AddressRange)
			if err == nil {
				ip := ipNet.IP.To4()
				if ip != nil && ip[0] == 10 && ip[1] >= 1 && ip[1] <= 254 && ip[2] == 0 {
					isTestCIDR = true
				}
			}
		}

		if isTestNetwork || isTestCIDR {
			// Delete nodes first to make network deletion synchronous
			nodes, _ := logic.GetNetworkNodes(netID)
			for _, node := range nodes {
				host, err := logic.GetHost(node.HostID.String())
				if err == nil {
					_ = logic.DissasociateNodeFromHost(&node, host)
				}
			}
			// Now delete network (should be synchronous since no nodes)
			done := make(chan struct{})
			go func() {
				_ = logic.DeleteNetwork(netID, true, done)
			}()
			// Wait for deletion to complete with timeout
			select {
			case <-done:
			case <-time.After(1 * time.Second):
				// Timeout - continue with next network
			}
		}
	}
}

// setupTestScenario creates a test network with a test node, peers, and ACL policies
func setupTestScenario(networkID models.NetworkID, numPeers, numPolicies, subnet int) (*models.Node, []*models.Node, error) {
	// Create test network with unique CIDR based on subnet parameter
	// Format: 10.{subnet}.0.0/24 to avoid conflicts
	addressRange := netpkg.IPv4(10, byte(subnet), 0, 0).String() + "/24"

	// Try to create network, retry if CIDR conflict (cleanup might still be running)
	var err error
	network := models.Network{
		NetID:        networkID.String(),
		AddressRange: addressRange,
	}

	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			// Check if a network with this CIDR already exists and delete it if it's a test network
			existingNetworks, _ := logic.GetNetworks()
			for _, existingNet := range existingNetworks {
				if existingNet.AddressRange == addressRange {
					// Delete the conflicting network if it's a test network
					if len(existingNet.NetID) > 12 && (existingNet.NetID[:12] == "test-network-" ||
						existingNet.NetID[:14] == "bench-network-" ||
						existingNet.NetID[:12] == "filter-bench") {
						// Delete nodes first
						nodes, _ := logic.GetNetworkNodes(existingNet.NetID)
						for _, node := range nodes {
							host, err := logic.GetHost(node.HostID.String())
							if err == nil {
								_ = logic.DissasociateNodeFromHost(&node, host)
							}
						}
						// Delete network (non-blocking)
						done := make(chan struct{}, 1)
						go func() {
							_ = logic.DeleteNetwork(existingNet.NetID, true, done)
						}()
						// Wait briefly for deletion
						select {
						case <-done:
						case <-time.After(200 * time.Millisecond):
						}
					}
				}
			}
			// Wait a bit before retrying
			time.Sleep(100 * time.Millisecond)
		}

		_, err = logic.CreateNetwork(network)
		if err == nil || database.IsEmptyRecord(err) {
			break // Success
		}
		if err.Error() != "network cidr already in use" {
			return nil, nil, err // Different error, don't retry
		}
		// CIDR conflict, will retry
	}
	if err != nil && !database.IsEmptyRecord(err) {
		return nil, nil, fmt.Errorf("failed to create network after retries: %w", err)
	}

	logic.CreateDefaultAclNetworkPolicies(networkID)

	fmt.Printf("  [setup] Creating test host\n")
	// Create test host
	key, err := wgtypes.GenerateKey()
	if err != nil {
		return nil, nil, err
	}
	hostID := uuid.New()
	// Generate unique MAC address for test host
	hostUUIDBytes := hostID[:6]
	testHostMAC := netpkg.HardwareAddr{
		hostUUIDBytes[0] | 0x02, // Set locally administered bit
		hostUUIDBytes[1],
		hostUUIDBytes[2],
		hostUUIDBytes[3],
		hostUUIDBytes[4],
		hostUUIDBytes[5],
	}
	host := models.Host{
		ID:         hostID,
		PublicKey:  key.PublicKey(),
		HostPass:   "testpass",
		OS:         "linux",
		Name:       "test-host-" + hostID.String()[:8],
		MacAddress: testHostMAC,
	}
	err = logic.CreateHost(&host)
	if err != nil && !database.IsEmptyRecord(err) {
		return nil, nil, err
	}

	fmt.Printf("  [setup] Creating test node\n")
	// Create test node - use subnet-based IP
	testNodeIP := netpkg.IPv4(10, byte(subnet), 0, 1)
	_, ipnet, _ := netpkg.ParseCIDR(testNodeIP.String() + "/32")
	testNode := &models.Node{
		CommonNode: models.CommonNode{
			ID:        uuid.New(),
			HostID:    host.ID,
			Network:   networkID.String(),
			Address:   *ipnet,
			Connected: true,
		},
		LastCheckIn: time.Now(),
	}
	err = logic.AssociateNodeToHost(testNode, &host)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to associate test node: %w", err)
	}
	fmt.Printf("  [setup] Test node created\n")

	fmt.Printf("  [setup] Creating %d peers\n", numPeers)
	// Create peers - each peer needs a unique host to avoid zombie detection
	peers := make([]*models.Node, 0, numPeers)
	for i := 0; i < numPeers; i++ {
		if i%10 == 0 || i < 3 {
			fmt.Printf("  [setup] Creating peer %d/%d\n", i+1, numPeers)
		}

		// Create a unique host for each peer to avoid zombie detection
		peerKey, _ := wgtypes.GenerateKey()
		peerHostID := uuid.New()
		// Generate unique MAC address from UUID to avoid zombie detection
		// Use first 6 bytes of UUID, set bit 1 of first byte for locally administered
		peerHostUUIDBytes := peerHostID[:6]
		uniqueMAC := netpkg.HardwareAddr{
			peerHostUUIDBytes[0] | 0x02, // Set locally administered bit
			peerHostUUIDBytes[1],
			peerHostUUIDBytes[2],
			peerHostUUIDBytes[3],
			peerHostUUIDBytes[4],
			peerHostUUIDBytes[5],
		}
		peerHost := models.Host{
			ID:         peerHostID,
			PublicKey:  peerKey.PublicKey(),
			HostPass:   "testpass",
			OS:         "linux",
			Name:       "peer-host-" + peerHostID.String()[:8],
			MacAddress: uniqueMAC,
		}

		fmt.Printf("  [setup]   Creating host for peer %d\n", i+1)
		err = logic.CreateHost(&peerHost)
		if err != nil && !database.IsEmptyRecord(err) {
			return nil, nil, fmt.Errorf("failed to create peer host %d: %w", i+1, err)
		}

		peerIP := netpkg.IPv4(10, byte(subnet), 0, byte(i+2))
		_, peerIPNet, _ := netpkg.ParseCIDR(peerIP.String() + "/32")
		peer := &models.Node{
			CommonNode: models.CommonNode{
				ID:        uuid.New(),
				HostID:    peerHost.ID, // Each peer has its own unique host
				Network:   networkID.String(),
				Address:   *peerIPNet,
				Connected: true,
			},
			LastCheckIn: time.Now(),
		}
		fmt.Printf("  [setup]   Associating node to host for peer %d\n", i+1)
		err = logic.AssociateNodeToHost(peer, &peerHost)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to associate peer %d: %w", i+1, err)
		}
		fmt.Printf("  [setup]   Peer %d created successfully\n", i+1)
		peers = append(peers, peer)
	}
	fmt.Printf("  [setup] All peers created\n")

	fmt.Printf("  [setup] Creating %d ACL policies\n", numPolicies)
	// Create custom ACL policies
	for i := 0; i < numPolicies; i++ {
		policy := models.Acl{
			ID:               uuid.New().String(),
			Name:             "Test Policy " + uuid.New().String()[:8],
			NetworkID:        networkID,
			RuleType:         models.DevicePolicy,
			Enabled:          true,
			Default:          false,
			Proto:            models.ALL,
			ServiceType:      models.Any,
			Port:             []string{},
			AllowedDirection: models.TrafficDirectionBi,
			CreatedBy:        "test",
			CreatedAt:        time.Now().UTC(),
		}

		// Create policies that match different nodes
		if i%3 == 0 {
			// Policy matching test node by ID
			policy.Src = []models.AclPolicyTag{
				{ID: models.NodeID, Value: testNode.ID.String()},
			}
			policy.Dst = []models.AclPolicyTag{
				{ID: models.NodeTagID, Value: "*"},
			}
		} else if i%3 == 1 {
			// Policy matching by wildcard
			policy.Src = []models.AclPolicyTag{
				{ID: models.NodeTagID, Value: "*"},
			}
			policy.Dst = []models.AclPolicyTag{
				{ID: models.NodeTagID, Value: "*"},
			}
		} else {
			// Policy matching specific peer
			if len(peers) > 0 {
				peerIdx := i % len(peers)
				policy.Src = []models.AclPolicyTag{
					{ID: models.NodeID, Value: peers[peerIdx].ID.String()},
				}
				policy.Dst = []models.AclPolicyTag{
					{ID: models.NodeID, Value: testNode.ID.String()},
				}
			}
		}

		err = logic.InsertAcl(policy)
		if err != nil {
			return nil, nil, err
		}
	}
	fmt.Printf("  [setup] Setup complete\n")

	return testNode, peers, nil
}

// cleanupTestScenario cleans up test data
func cleanupTestScenario(networkID models.NetworkID, testNode *models.Node, peers []*models.Node) {
	// Cleanup nodes
	if testNode != nil {
		_ = database.DeleteRecord(database.NODES_TABLE_NAME, testNode.ID.String())
	}
	for _, peer := range peers {
		_ = database.DeleteRecord(database.NODES_TABLE_NAME, peer.ID.String())
	}

	// Cleanup network
	_ = database.DeleteRecord(database.NETWORKS_TABLE_NAME, networkID.String())

	// Cleanup ACLs
	acls, _ := logic.ListAclsByNetwork(networkID)
	for _, acl := range acls {
		if !acl.Default {
			_ = database.DeleteRecord(database.ACLS_TABLE_NAME, acl.ID)
		}
	}

	// Clear caches
	if servercfg.CacheEnabled() {
		logic.ClearNodeCache()
	}
}
