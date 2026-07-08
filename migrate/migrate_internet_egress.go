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
	"gorm.io/datatypes"
)

// migrateInternetEgress:
// 1) backfills Type=internet on egresses with Range="*"
// 2) creates internet-type egress resources for legacy IsInternetGateway nodes
// 3) sets SelectedInternetEgressID on former IGW clients
func migrateInternetEgress() {
	ctx := db.WithContext(context.TODO())
	logger.Log(1, "migration: migrating internet gateways to egress type internet")
	defer logger.Log(1, "migration: completed internet gateway to egress migration")

	backfillInternetEgressTypes(ctx)

	nodes, err := logic.GetAllNodes()
	if err != nil {
		logger.Log(0, "migration: failed to list nodes for internet egress migrate:", err.Error())
		return
	}

	// Map routing-node-id -> egress ID for internet egresses created/found in this run.
	routingToEgress := make(map[string]string)

	for _, node := range nodes {
		if !node.IsInternetGateway {
			continue
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
		clientIDs := collectLegacyIGWClients(node, nodes)
		if len(clientIDs) > 0 {
			createMigrationInternetEgressACL(node.Network, e, clientIDs)
		}
	}

	for _, node := range nodes {
		egressID := ""
		if node.SelectedInternetEgressID != "" {
			continue
		}
		if node.InternetGwID != "" {
			if id, ok := routingToEgress[node.InternetGwID]; ok {
				egressID = id
			} else if e, err := logic.FindInternetEgressByRoutingNode(ctx, node.Network, node.InternetGwID); err == nil {
				egressID = e.ID
				routingToEgress[node.InternetGwID] = e.ID
			}
		}
		if egressID == "" {
			continue
		}
		n := node
		n.SelectedInternetEgressID = egressID
		if err := logic.UpsertNode(&n); err != nil {
			logger.Log(0, "migration: failed to set selected internet egress on node", n.ID.String(), err.Error())
		}
	}
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

func createMigrationInternetEgressACL(network string, e schema.Egress, clientNodeIDs []string) {
	src := make([]models.AclPolicyTag, 0, len(clientNodeIDs))
	for _, id := range clientNodeIDs {
		src = append(src, models.AclPolicyTag{ID: models.NodeID, Value: id})
	}
	acl := models.Acl{
		ID:               fmt.Sprintf("%s.inet-egress-%s", network, e.ID),
		Name:             fmt.Sprintf("Internet egress %s (migrated)", e.Name),
		NetworkID:        schema.NetworkID(network),
		RuleType:         models.DevicePolicy,
		Default:          false,
		Enabled:          true,
		Src:              src,
		Dst:              []models.AclPolicyTag{{ID: models.EgressID, Value: e.ID}},
		Proto:            models.ALL,
		AllowedDirection: models.TrafficDirectionBi,
	}
	if err := logic.InsertAcl(acl); err != nil {
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
