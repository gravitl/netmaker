package migrate

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
)

// migrateInternetEgress:
// 1) backfills Type=internet on egresses with Range="*"
// 2) creates internet-type egress resources for legacy IsInternetGateway nodes
// 3) clears obsolete central relayed_igw_clients rosters on IGW routing nodes
// 4) sets SelectedInternetEgressID + RelayedBy/IsIGWClient on former IGW client nodes
// 5) sets SelectedInternetEgressID on ext clients attached to those inet gws
//    (enables "use gateway as exit node"); recovers configs if a prior run cleared
//    IsInternetGateway before stamping selections
// 6) clears IsInternetGateway only on routing nodes whose ext-client migration succeeded
func migrateInternetEgress() {
	ctx := db.WithContext(context.TODO())
	logger.Log(1, "migration: migrating internet gateways to egress type internet")
	defer logger.Log(1, "migration: completed internet gateway to egress migration")

	backfillInternetEgressTypes(ctx)

	nodes, err := logic.GetAllNodes(ctx)
	if err != nil {
		logger.Log(0, "migration: failed to list nodes for internet egress migrate:", err.Error())
		return
	}

	// Map routing-node-id -> egress ID for internet egresses created/found in this run.
	routingToEgress := make(map[string]string)
	// Clients to migrate per routing node (from InetNodeReq / InternetGwID), for selection after roster clear.
	legacyClientsByRoutingNode := make(map[string][]string)

	for _, node := range nodes {
		if !node.IsInternetGateway {
			continue
		}
		clientIDs := collectLegacyIGWClients(node, nodes)
		if len(clientIDs) > 0 {
			legacyClientsByRoutingNode[node.ID.String()] = clientIDs
		}

		if existing, err := logic.FindInternetEgressByRoutingNode(ctx, node.Network, node.ID.String()); err == nil && existing != nil {
			routingToEgress[node.ID.String()] = existing.ID
			continue
		}

		host := &schema.Host{ID: node.HostID}
		_ = host.Get(ctx)
		name := node.ID.String() + "-internet"
		if host.Name != "" {
			name = host.Name + "-internet"
		}

		e := schema.Egress{
			ID:        uuid.New().String(),
			Name:      name,
			Network:   node.Network,
			Type:      schema.EgressTypeInternet,
			Range:     "*",
			Nat:       true,
			Mode:      schema.DirectNAT,
			Nodes:     datatypes.JSONMap{node.ID.String(): 256},
			Tags:      make(datatypes.JSONMap),
			Status:    true,
			CreatedBy: "migration",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
			TenantID:  getNodeTenantID(ctx, node),
		}
		if err := e.Create(ctx); err != nil {
			logger.Log(0, "migration: failed to create internet egress for node", node.ID.String(), err.Error())
			continue
		}
		routingToEgress[node.ID.String()] = e.ID
		logger.Log(1, fmt.Sprintf("migration: created internet egress %s for node %s", e.ID, node.ID.String()))

		// Preserve access for previously assigned clients with an allow ACL when default policies are off.
		if len(clientIDs) > 0 {
			createMigrationInternetEgressACL(ctx, node, e, clientIDs)
		}
	}

	// Clear obsolete central rosters only on routing nodes that mapped to an internet egress.
	clearLegacyInetNodeClientLists(ctx, nodes, routingToEgress)
	migrateNodeInternetEgressSelections(ctx, nodes, routingToEgress, legacyClientsByRoutingNode)
	// Enable "use gateway as exit node" on configs that were attached to legacy inet gws.
	// Only clear IsInternetGateway for gateways whose ext-client saves succeeded so a
	// failed/partial run can retry while the legacy flag is still set.
	clearable := migrateExtClientInternetEgressSelections(ctx, nodes, routingToEgress)
	clearLegacyIsInternetGateway(ctx, clearable)
}

func migrateNodeInternetEgressSelections(ctx context.Context, nodes []models.Node, routingToEgress map[string]string, legacyClientsByRoutingNode map[string][]string) {
	nodesByID := make(map[string]models.Node, len(nodes))
	for _, node := range nodes {
		nodesByID[node.ID.String()] = node
	}

	migrateOne := func(n models.Node, egressID string) {
		// Skip only when a different exit is already selected; re-apply when the same
		// egress is selected so RelayedBy/IsIGWClient pairing is restored.
		if n.SelectedInternetEgressID != "" && n.SelectedInternetEgressID != egressID {
			return
		}
		if err := logic.SetNodeSelectedInternetEgress(&n, egressID, n.UseTcpUplink); err != nil {
			logger.Log(0, "migration: failed to set selected internet egress on node", n.ID.String(), err.Error())
		}
	}

	for routingNodeID, clientIDs := range legacyClientsByRoutingNode {
		network := ""
		if gw, ok := nodesByID[routingNodeID]; ok {
			network = gw.Network
		}
		egressID := internetEgressIDForRoutingNode(ctx, routingNodeID, network, routingToEgress)
		if egressID == "" {
			continue
		}
		for _, clientID := range clientIDs {
			n, ok := nodesByID[clientID]
			if !ok {
				continue
			}
			migrateOne(n, egressID)
		}
	}

	for _, node := range nodes {
		if node.SelectedInternetEgressID != "" {
			continue
		}
		egressID := internetEgressIDForRoutingNode(ctx, node.InternetGwID, node.Network, routingToEgress)
		if egressID == "" {
			continue
		}
		migrateOne(node, egressID)
	}
}

// migrateExtClientInternetEgressSelections sets SelectedInternetEgressID on config
// files attached to legacy internet gateways (enables "use gateway as exit node").
// Returns the subset of routingToEgress that is safe to clear IsInternetGateway on
// (all attached configs for that gateway were migrated successfully or needed none).
func migrateExtClientInternetEgressSelections(ctx context.Context, nodes []models.Node, routingToEgress map[string]string) map[string]string {
	clearable := make(map[string]string, len(routingToEgress))
	for id, egressID := range routingToEgress {
		clearable[id] = egressID
	}

	nodesByID := make(map[string]models.Node, len(nodes))
	for _, node := range nodes {
		nodesByID[node.ID.String()] = node
	}

	// Also recover configs when a prior run created the internet egress and cleared
	// IsInternetGateway but failed to stamp SelectedInternetEgressID (e.g. missing
	// tenant scope on SaveExtClient). Only when every attached config still has an
	// empty selection — otherwise new opt-out configs would be force-enabled.
	recoverRoutingToEgressForUnmigratedConfigs(ctx, nodesByID, routingToEgress)

	extclients, err := logic.GetAllExtClients(ctx)
	if err != nil {
		logger.Log(0, "migration: failed to list ext clients for internet egress migrate:", err.Error())
		// Do not clear any IsInternetGateway flags; retry next startup.
		return nil
	}

	failedGateways := make(map[string]struct{})
	for _, client := range extclients {
		if client.SelectedInternetEgressID != "" {
			continue
		}
		egressID, ok := routingToEgress[client.IngressGatewayID]
		if !ok {
			continue
		}

		tenantID := ""
		if gw, ok := nodesByID[client.IngressGatewayID]; ok {
			tenantID = gw.TenantID
			if tenantID == "" {
				tenantID = getNodeTenantID(ctx, gw)
			}
		}
		if tenantID == "" {
			logger.Log(0, "migration: missing tenant for ext client", client.ClientID, "gateway", client.IngressGatewayID)
			failedGateways[client.IngressGatewayID] = struct{}{}
			continue
		}

		c := client
		c.SelectedInternetEgressID = egressID
		clientCtx := scope.WithContext(ctx, scope.TenantScope, tenantID)
		if err := logic.SaveExtClient(clientCtx, &c); err != nil {
			logger.Log(0, "migration: failed to set selected internet egress on ext client", c.ClientID, err.Error())
			failedGateways[client.IngressGatewayID] = struct{}{}
			continue
		}
		logger.Log(1, fmt.Sprintf("migration: enabled use-gw-as-exit on config %s (egress %s)", c.ClientID, egressID))
	}

	for gwID := range failedGateways {
		delete(clearable, gwID)
	}
	return clearable
}

// recoverRoutingToEgressForUnmigratedConfigs adds migration-created internet egresses
// to routingToEgress when none of the gateway's configs have SelectedInternetEgressID
// yet. That covers upgrades where IsInternetGateway was cleared before configs were stamped.
func recoverRoutingToEgressForUnmigratedConfigs(ctx context.Context, nodesByID map[string]models.Node, routingToEgress map[string]string) {
	egs, err := (&schema.Egress{}).ListAll(ctx)
	if err != nil {
		logger.Log(0, "migration: failed to list egresses for ext client recovery:", err.Error())
		return
	}

	extclients, err := logic.GetAllExtClients(ctx)
	if err != nil {
		logger.Log(0, "migration: failed to list ext clients for recovery:", err.Error())
		return
	}

	clientsByGateway := make(map[string][]models.ExtClient)
	for _, client := range extclients {
		if client.IngressGatewayID == "" {
			continue
		}
		clientsByGateway[client.IngressGatewayID] = append(clientsByGateway[client.IngressGatewayID], client)
	}

	for _, eg := range egs {
		if eg.CreatedBy != "migration" || !logic.IsEgressInternetGateway(eg) || !eg.Status {
			continue
		}
		for routingNodeID := range eg.Nodes {
			if _, already := routingToEgress[routingNodeID]; already {
				continue
			}
			if _, ok := nodesByID[routingNodeID]; !ok {
				continue
			}
			attached := clientsByGateway[routingNodeID]
			if len(attached) == 0 {
				continue
			}
			if !allExtClientsMissingInternetEgressSelection(attached) {
				continue
			}
			routingToEgress[routingNodeID] = eg.ID
			logger.Log(1, fmt.Sprintf("migration: recovering ext-client exit selection for gateway %s via egress %s", routingNodeID, eg.ID))
		}
	}
}

func allExtClientsMissingInternetEgressSelection(clients []models.ExtClient) bool {
	for _, client := range clients {
		if client.SelectedInternetEgressID != "" {
			return false
		}
	}
	return true
}

// clearLegacyInetNodeClientLists clears RelayedIGWClients (source of InetNodeReq.InetNodeClientIDs)
// on routing nodes that successfully mapped to an internet egress (keys of routingToEgress).
// Per-client RelayedBy/IsIGWClient are preserved and re-applied afterwards via
// SetNodeSelectedInternetEgress → AssignGateway.
func clearLegacyInetNodeClientLists(ctx context.Context, nodes []models.Node, routingToEgress map[string]string) {
	for _, node := range nodes {
		if _, ok := routingToEgress[node.ID.String()]; !ok {
			continue
		}
		_node := &schema.Node{ID: node.ID.String()}
		if err := _node.Get(ctx); err != nil {
			logger.Log(0, "migration: failed to load node to clear igw client list:", node.ID.String(), err.Error())
			continue
		}
		if len(_node.RelayedIGWClients) == 0 && len(node.InetNodeReq.InetNodeClientIDs) == 0 {
			continue
		}
		if err := db.FromContext(ctx).Model(&schema.Node{}).
			Where("id = ?", node.ID.String()).
			Update("relayed_igw_clients", datatypes.JSONMap{}).Error; err != nil {
			logger.Log(0, "migration: failed to clear relayed_igw_clients on node", node.ID.String(), err.Error())
			continue
		}
		logger.Log(1, fmt.Sprintf("migration: cleared legacy inet gw client list on node %s", node.ID.String()))
	}
}

// clearLegacyIsInternetGateway clears the legacy IsInternetGateway flag on routing nodes
// that were successfully migrated to an internet-type egress resource.
func clearLegacyIsInternetGateway(ctx context.Context, routingToEgress map[string]string) {
	for routingNodeID := range routingToEgress {
		if err := db.FromContext(ctx).Model(&schema.Node{}).
			Where("id = ?", routingNodeID).
			Update("is_internet_gateway", false).Error; err != nil {
			logger.Log(0, "migration: failed to clear is_internet_gateway on node", routingNodeID, err.Error())
			continue
		}
		logger.Log(1, fmt.Sprintf("migration: cleared legacy is_internet_gateway on node %s", routingNodeID))
	}
}

func internetEgressIDForRoutingNode(ctx context.Context, routingNodeID, network string, routingToEgress map[string]string) string {
	if routingNodeID == "" {
		return ""
	}
	if id, ok := routingToEgress[routingNodeID]; ok {
		return id
	}
	e, err := logic.FindInternetEgressByRoutingNode(ctx, network, routingNodeID)
	if err != nil || e == nil {
		return ""
	}
	routingToEgress[routingNodeID] = e.ID
	return e.ID
}

func backfillInternetEgressTypes(ctx context.Context) {
	egs, err := (&schema.Egress{}).ListAll(ctx)
	if err != nil {
		logger.Log(0, "migration: failed to list egresses for type backfill:", err.Error())
		return
	}
	for _, eg := range egs {
		needUpdate := false
		updates := map[string]any{}
		if eg.Type == "" {
			switch {
			case eg.Range == "*":
				eg.Type = schema.EgressTypeInternet
			case eg.PresetID != "":
				eg.Type = schema.EgressTypeApp
			case len(eg.Domains) > 0:
				eg.Type = schema.EgressTypeDomain
			default:
				eg.Type = schema.EgressTypeCIDR
			}
			updates["egress_type"] = eg.Type
			needUpdate = true
		}
		if eg.Range == "*" && eg.Type != schema.EgressTypeInternet {
			updates["egress_type"] = schema.EgressTypeInternet
			needUpdate = true
		}
		if needUpdate {
			if err := db.FromContext(ctx).Table(eg.Table()).Where("id = ?", eg.ID).Updates(updates).Error; err != nil {
				logger.Log(0, "migration: failed to backfill egress type:", eg.ID, err.Error())
			}
		}
	}
}

func collectLegacyIGWClients(gw models.Node, all []models.Node) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(id string) {
		if id == "" || id == gw.ID.String() {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	for _, id := range gw.InetNodeReq.InetNodeClientIDs {
		add(id)
	}
	for _, n := range all {
		if n.Network == gw.Network && n.InternetGwID == gw.ID.String() {
			add(n.ID.String())
		}
	}
	return out
}

func createMigrationInternetEgressACL(ctx context.Context, node models.Node, e schema.Egress, clientNodeIDs []string) {
	tenantID := node.TenantID
	if tenantID == "" {
		tenantID = getNodeTenantID(ctx, node)
	}
	if tenantID == "" {
		logger.Log(0, "migration: missing tenant for internet egress ACL on node", node.ID.String())
		return
	}
	aclCtx := scope.WithContext(ctx, scope.TenantScope, tenantID)
	aclID := fmt.Sprintf("%s.inet-egress-%s", node.Network, e.ID)
	if _, err := logic.GetAcl(aclCtx, aclID); err == nil {
		return
	}
	src := make([]models.AclPolicyTag, 0, len(clientNodeIDs))
	for _, id := range clientNodeIDs {
		src = append(src, models.AclPolicyTag{ID: models.NodeID, Value: id})
	}
	acl := models.Acl{
		ID:               aclID,
		Name:             fmt.Sprintf("Internet egress %s (migrated)", e.Name),
		NetworkID:        schema.NetworkID(node.Network),
		RuleType:         models.DevicePolicy,
		Default:          false,
		Enabled:          true,
		Src:              src,
		Dst:              []models.AclPolicyTag{{ID: models.EgressID, Value: e.ID}},
		Proto:            models.ALL,
		AllowedDirection: models.TrafficDirectionBi,
	}
	if err := logic.InsertAcl(aclCtx, acl); err != nil {
		logger.Log(0, "migration: failed to create internet egress ACL:", e.ID, err.Error())
	}
}

func getNodeTenantID(ctx context.Context, node models.Node) string {
	_node := &schema.Node{ID: node.ID.String()}
	if err := _node.Get(ctx); err == nil && _node.TenantID != "" {
		return _node.TenantID
	}
	defaultTenant := &schema.Tenant{}
	if err := defaultTenant.GetDefault(ctx); err == nil {
		return defaultTenant.ID
	}
	return ""
}
