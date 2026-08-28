package schema

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/db/expr"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
)

const nodesTable = "nodes_v1"

const (
	// NODE_DELETE - delete node action
	NODE_DELETE = "delete"
	// NODE_IS_PENDING - node pending status
	NODE_IS_PENDING = "pending"
	// NODE_NOOP - node no op action
	NODE_NOOP = "noop"
	// NODE_FORCE_UPDATE - indicates a node should pull all changes
	NODE_FORCE_UPDATE = "force"
)

type NodeStatus string

const (
	OnlineSt     NodeStatus = "online"
	OfflineSt    NodeStatus = "offline"
	WarningSt    NodeStatus = "warning"
	ErrorSt      NodeStatus = "error"
	UnKnown      NodeStatus = "unknown"
	Disconnected NodeStatus = "disconnected"
)

type Node struct {
	ID                                string                                `gorm:"primaryKey" json:"id"`
	TenantID                          string                                `gorm:"default:'';index" json:"tenant_id"`
	HostID                            string                                `gorm:"not null;index" json:"host_id"`
	Host                              *Host                                 `gorm:"foreignKey:HostID;constraint:OnDelete:CASCADE" json:"host,omitempty"`
	NetworkID                         string                                `gorm:"not null;index" json:"network_id"`
	Network                           *Network                              `gorm:"foreignKey:NetworkID;constraint:OnDelete:CASCADE" json:"network,omitempty"`
	Address                           string                                `json:"address"`
	Address6                          string                                `json:"address6"`
	Connected                         bool                                  `json:"connected"`
	Action                            string                                `json:"action"`
	Status                            NodeStatus                            `json:"status"`
	PendingDelete                     bool                                  `json:"pending_delete"`
	AutoAssignGateway                 bool                                  `json:"auto_assign_gateway"`
	IsGateway                         bool                                  `json:"is_gateway"`
	IsAutoRelay                       string                                `json:"is_auto_relay"`
	IsInternetGateway                 bool                                  `json:"is_internet_gateway"`
	AdditionalGatewayEndpoints        datatypes.JSONSlice[string]           `json:"additional_gateway_endpoints"`
	RelayedClients                    datatypes.JSONMap                     `json:"relayed_clients"`
	RelayedIGWClients                 datatypes.JSONMap                     `json:"relayed_igw_clients"`
	RelayedByNodeID                   *string                               `json:"relayed_by_node_id"`
	IsIGWClient                       bool                                  `json:"is_igw_client"`
	// UseTcpUplink: assigned/relayed node opts into TCP uplink to its gateway (requires host TcpProxyEnabled).
	UseTcpUplink bool `json:"use_tcp_uplink"`
	// SelectedInternetEgressID is the internet-type egress this node uses as its exit node (empty = none).
	SelectedInternetEgressID          string                                `json:"selected_internet_egress_id"`
	AutoRelayedPeers                  datatypes.JSONType[map[string]string] `json:"auto_relayed_peers"`
	Tags                              datatypes.JSONMap                     `json:"tags"`
	PostureCheckSeverity              Severity                              `json:"posture_check_severity"`
	PostureCheckLastEvaluationCycleID string                                `json:"posture_check_last_evaluation_cycle_id"`
	PostureCheckLastEvaluatedAt       time.Time                             `json:"posture_check_last_evaluated_at"`
	Metadata                          string                                `json:"metadata"`
	LastCheckIn                       time.Time                             `json:"last_check_in"`
	ExpirationDateTime                time.Time                             `json:"expiration_date_time"`
	CreatedAt                         time.Time                             `json:"created_at"`
	UpdatedAt                         time.Time                             `json:"updated_at"`
}

func (n *Node) TableName() string {
	return nodesTable
}

func (n *Node) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).Create(n).Error
}

func (n *Node) Get(ctx context.Context, options ...dbtypes.Option) error {
	query := db.FromContext(ctx).Model(&Node{})
	for _, opt := range options {
		query = opt(query)
	}
	return query.Where("id = ?", n.ID).First(n).Error
}

func (n *Node) Exists(ctx context.Context) (bool, error) {
	var exists bool
	err := db.FromContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM nodes_v1 WHERE id = ?)",
		n.ID,
	).Scan(&exists).Error
	return exists, err
}

func (n *Node) GetByHostAndNetwork(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("host_id = ? AND network_id = ?", n.HostID, n.NetworkID).
		First(n).
		Error
}

func (n *Node) GetByNetworkAndAddress(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("network_id = ? AND address = ?", n.NetworkID, n.Address).
		First(n).
		Error
}

func (n *Node) GetByNetworkAndAddress6(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("network_id = ? AND address6 = ?", n.NetworkID, n.Address6).
		First(n).
		Error
}

func (n *Node) Update(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).Where("id = ?", n.ID).Updates(n).Error
}

func (n *Node) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Save(n).Error
}

func (n *Node) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).Where("id = ?", n.ID).Delete(n).Error
}

func (n *Node) DeleteAll(ctx context.Context) error {
	if tenantID := scope.ID(ctx); tenantID != "" {
		return db.FromContext(ctx).Exec("DELETE FROM nodes_v1 WHERE tenant_id = ?", tenantID).Error
	}
	return db.FromContext(ctx).Exec("DELETE FROM nodes_v1").Error
}

func (n *Node) ListAll(ctx context.Context, options ...dbtypes.Option) ([]Node, error) {
	var nodes []Node
	query := db.FromContext(ctx).Model(&Node{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		options = append(options, dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", nodesTable), tenantID))
	}
	for _, opt := range options {
		query = opt(query)
	}
	err := query.Find(&nodes).Error
	return nodes, err
}

// ListByIDs fetches nodes whose IDs are in the given slice using a single
// `WHERE id IN (?)` query. Returns an empty slice when ids is empty.
func (n *Node) ListByIDs(ctx context.Context, ids []string, options ...dbtypes.Option) ([]Node, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	query := db.FromContext(ctx).Model(&Node{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		options = append(options, dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", nodesTable), tenantID))
	}
	for _, opt := range options {
		query = opt(query)
	}
	var nodes []Node
	err := query.Where("id IN ?", ids).Find(&nodes).Error
	return nodes, err
}

func (n *Node) Count(ctx context.Context, options ...dbtypes.Option) (int, error) {
	var count int64
	query := db.FromContext(ctx).Model(&Node{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		options = append(options, dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", nodesTable), tenantID))
	}
	for _, opt := range options {
		query = opt(query)
	}
	err := query.Count(&count).Error
	return int(count), err
}

func (n *Node) UpsertViolations(ctx context.Context, violations []PostureCheckViolation) error {
	if len(violations) > 0 {
		for i := range violations {
			if violations[i].TenantID == "" {
				violations[i].TenantID = n.TenantID
			}
		}

		err := db.FromContext(ctx).Model(&PostureCheckViolation{}).Create(&violations).Error
		if err != nil {
			return err
		}
	}

	return db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		Update("posture_check_severity", n.PostureCheckSeverity).
		Update("posture_check_last_evaluation_cycle_id", n.PostureCheckLastEvaluationCycleID).
		Update("posture_check_last_evaluated_at", n.PostureCheckLastEvaluatedAt).
		Error
}

func (n *Node) ListViolations(ctx context.Context) ([]PostureCheckViolation, error) {
	var violations []PostureCheckViolation
	query := db.FromContext(ctx).Model(&PostureCheckViolation{}).
		Where("node_id = ? AND evaluation_cycle_id = ?", n.ID, n.PostureCheckLastEvaluationCycleID)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", postureCheckViolationsTable), tenantID)(query)
	}
	err := query.Find(&violations).Error
	return violations, err
}

func (n *Node) DeleteViolations(ctx context.Context) error {
	query := db.FromContext(ctx).Model(&PostureCheckViolation{}).
		Where("node_id = ?", n.ID)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", postureCheckViolationsTable), tenantID)(query)
	}
	return query.Delete(&PostureCheckViolation{}).Error
}

func (n *Node) UpdateConnectedStatus(ctx context.Context, options ...dbtypes.Option) error {
	query := db.FromContext(ctx).Model(&Node{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		options = append(options, dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", nodesTable), tenantID))
	}
	for _, opt := range options {
		query = opt(query)
	}
	if n.ID != "" {
		query = query.Where("id = ?", n.ID)
	}

	updates := make(map[string]interface{})
	updates["connected"] = n.Connected
	updates["status"] = n.Status
	if n.Connected {
		updates["last_check_in"] = n.LastCheckIn
	}
	return query.Updates(updates).Error
}

// UpdateStatus updates only the status column for a single node by ID.
// Lighter than Upsert: no preloads, no full row write, no association handling.
func (n *Node) UpdateStatus(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		Update("status", n.Status).
		Error
}

// MarkStaleOffline sets status=offline for every node whose last_check_in is
// older than the given threshold and which is not already offline, not
// pending deletion, and not flagged for the delete action. Returns the number
// of rows updated. Issues exactly one SQL statement.
func (n *Node) MarkStaleOffline(ctx context.Context, threshold time.Time) (int64, error) {
	result := db.FromContext(ctx).Model(&Node{}).
		Where("last_check_in < ?", threshold).
		Where("status <> ?", OfflineSt).
		Where("pending_delete = ?", false).
		Where("action <> ?", NODE_DELETE).
		Update("status", OfflineSt)
	return result.RowsAffected, result.Error
}

func (n *Node) MarkForDeletion(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		Update("pending_delete", true).
		Update("action", NODE_DELETE).
		Error
}

func (n *Node) SetInternetGateway(ctx context.Context) error {
	err := db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		UpdateColumn("is_internet_gateway", n.IsInternetGateway).
		UpdateColumn("relayed_clients", expr.Merge("relayed_clients", n.RelayedClients)).
		UpdateColumn("relayed_igw_clients", expr.Merge("relayed_igw_clients", n.RelayedIGWClients)).
		Error
	if err != nil {
		return err
	}

	relayedIGWClients := make([]string, 0, len(n.RelayedIGWClients))
	for relayedIGWClientID := range n.RelayedIGWClients {
		relayedIGWClients = append(relayedIGWClients, relayedIGWClientID)
	}

	return db.FromContext(ctx).Model(&Node{}).
		Where("id IN ?", relayedIGWClients).
		UpdateColumn("is_igw_client", true).
		UpdateColumn("relayed_by_node_id", n.ID).
		Error
}

func (n *Node) UpdateRelayingNode(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		Update("relayed_by_node_id", n.RelayedByNodeID).
		Error
}

func (n *Node) UpdateRelayedClients(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		Update("relayed_clients", n.RelayedClients).
		Error
}

func (n *Node) AssignTag(ctx context.Context, tag string, options ...dbtypes.Option) error {
	query := db.FromContext(ctx).Model(&Node{})
	for _, opt := range options {
		query = opt(query)
	}
	if n.ID != "" {
		query = query.Where("id = ?", n.ID)
	}

	return query.UpdateColumn(
		"tags",
		expr.Merge(
			"tags",
			map[string]interface{}{
				tag: struct{}{},
			},
		),
	).Error
}

func (n *Node) UnassignTag(ctx context.Context, tag string, options ...dbtypes.Option) error {
	query := db.FromContext(ctx).Model(&Node{})
	for _, opt := range options {
		query = opt(query)
	}
	if n.ID != "" {
		query = query.Where("id = ?", n.ID)
	}

	return query.UpdateColumn("tags", expr.Remove("tags", tag)).Error
}

func (n *Node) UpdateLastCheckIn(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		Update("last_check_in", n.LastCheckIn).
		Error
}

func (n *Node) SetRelayedClients(ctx context.Context) error {
	err := db.FromContext(ctx).Model(&Node{}).
		Where("relayed_by_node_id = ?", n.ID).
		Update("relayed_by_node_id", nil).
		Error
	if err != nil {
		return err
	}

	err = db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		Update("relayed_clients", n.RelayedClients).
		Error
	if err != nil {
		return err
	}

	if len(n.RelayedClients) > 0 {
		clientIDs := make([]string, 0, len(n.RelayedClients))
		for clientID := range n.RelayedClients {
			clientIDs = append(clientIDs, clientID)
		}

		err = db.FromContext(ctx).Model(&Node{}).
			Where("id IN ?", clientIDs).
			Update("relayed_by_node_id", n.ID).
			Error
		if err != nil {
			return err
		}
	}

	return nil
}

func (n *Node) SetAutoAssignGateway(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		UpdateColumn("auto_assign_gateway", n.AutoAssignGateway).
		Error
}

func (n *Node) ResetAutoAssignGateway(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		Update("auto_assign_gateway", n.AutoAssignGateway).
		Error
}

func (n *Node) ResetAutoRelayedPeers(ctx context.Context) error {
	if n.NetworkID == "" {
		return fmt.Errorf("network_id not set")
	}

	err := db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		Update("auto_relayed_peers", datatypes.JSONMap{}).
		Error
	if err != nil {
		return err
	}

	return db.FromContext(ctx).Model(&Node{}).
		Where("network_id = ?", n.NetworkID).
		Where(expr.WhereNotNull("auto_relayed_peers", n.ID)).
		UpdateColumn("auto_relayed_peers", expr.Remove("auto_relayed_peers", n.ID)).
		Error
}

// DefaultTcpProxyListenPort is used when TcpProxyEnabled is set with listen port <= 0.
const DefaultTcpProxyListenPort = 443

// TcpProxyClientPortProxy is the default client-facing WSS port published when
// TLS mode is proxy (external termination). Override at runtime with the
// TCP_PROXY_PUBLIC_PORT server environment variable.
const TcpProxyClientPortProxy = 443

// TcpProxyTLSMode values (host/node). Empty defaults to selfsigned on clients.
const (
	TcpProxyTLSModeSelfSigned = "selfsigned"
	TcpProxyTLSModeProxy      = "proxy"
)

// NormaliseTcpProxyTLSMode returns a valid mode; empty → selfsigned.
func NormaliseTcpProxyTLSMode(mode string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", TcpProxyTLSModeSelfSigned:
		return TcpProxyTLSModeSelfSigned, nil
	case TcpProxyTLSModeProxy:
		return TcpProxyTLSModeProxy, nil
	default:
		return "", fmt.Errorf("unsupported uplink TLS mode: %s", mode)
	}
}

// NormaliseTcpProxyPublicHostname cleans a public hostname for WSS publish.
// Strips schemes/paths; rejects values that look like URLs with paths or empty hosts.
func NormaliseTcpProxyPublicHostname(raw string) (string, error) {
	h := strings.TrimSpace(raw)
	if h == "" {
		return "", nil
	}
	lower := strings.ToLower(h)
	for _, pfx := range []string{"wss://", "ws://", "https://", "http://"} {
		if strings.HasPrefix(lower, pfx) {
			h = h[len(pfx):]
			break
		}
	}
	if i := strings.IndexAny(h, "/?"); i >= 0 {
		h = h[:i]
	}
	h = strings.TrimSpace(h)
	if h == "" {
		return "", fmt.Errorf("invalid tcp_proxy_public_hostname")
	}
	// Drop accidental port; public port is published separately.
	if host, _, err := net.SplitHostPort(h); err == nil {
		h = host
	} else if strings.HasPrefix(h, "[") && strings.Contains(h, "]") {
		// leave bracketed IPv6 literals as-is without port
	}
	h = strings.TrimSpace(h)
	if h == "" || strings.ContainsAny(h, " /\\") {
		return "", fmt.Errorf("invalid tcp_proxy_public_hostname")
	}
	return h, nil
}

func (n *Node) AssignGateway(ctx context.Context) error {
	if n.NetworkID == "" {
		return fmt.Errorf("network_id not set")
	}

	err := db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		Updates(map[string]interface{}{
			"relayed_by_node_id": n.RelayedByNodeID,
			"is_igw_client":      n.IsIGWClient,
			"use_tcp_uplink":     n.UseTcpUplink,
		}).Error
	if err != nil {
		return err
	}

	// remove this node from relayed_clients of any other node in the network.
	err = db.FromContext(ctx).Model(&Node{}).
		Where("network_id = ?", n.NetworkID).
		Where(expr.WhereNotNull("relayed_clients", n.ID)).
		UpdateColumn("relayed_clients", expr.Remove("relayed_clients", n.ID)).
		Error
	if err != nil {
		return err
	}

	err = db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", *n.RelayedByNodeID).
		UpdateColumn("relayed_clients", expr.Merge("relayed_clients", map[string]interface{}{
			n.ID: struct{}{},
		})).Error
	if err != nil {
		return err
	}

	if n.IsIGWClient {
		// remove this node from relayed_igw_clients of any other node in the network.
		err = db.FromContext(ctx).Model(&Node{}).
			Where("network_id = ?", n.NetworkID).
			Where(expr.WhereNotNull("relayed_igw_clients", n.ID)).
			UpdateColumn("relayed_igw_clients", expr.Remove("relayed_igw_clients", n.ID)).
			Error
		if err != nil {
			return err
		}

		return db.FromContext(ctx).Model(&Node{}).
			Where("id = ?", *n.RelayedByNodeID).
			UpdateColumn("relayed_igw_clients", expr.Merge("relayed_igw_clients", map[string]interface{}{
				n.ID: struct{}{},
			})).Error
	}

	return nil
}

func (n *Node) UnassignGateway(ctx context.Context) error {
	n.UseTcpUplink = false
	if n.NetworkID == "" {
		existing := &Node{ID: n.ID}
		if err := existing.Get(ctx); err == nil {
			n.NetworkID = existing.NetworkID
		}
	}
	err := db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		Updates(map[string]interface{}{
			"relayed_by_node_id": n.RelayedByNodeID,
			"is_igw_client":      n.IsIGWClient,
			"use_tcp_uplink":     false,
		}).Error
	if err != nil {
		return err
	}

	// Prefer network-scoped remove; if NetworkID is still unknown, scrub the key
	// from any node that still lists this client so orphans cannot linger.
	relayedClientsQ := db.FromContext(ctx).Model(&Node{}).
		Where(expr.WhereNotNull("relayed_clients", n.ID))
	relayedIGWQ := db.FromContext(ctx).Model(&Node{}).
		Where(expr.WhereNotNull("relayed_igw_clients", n.ID))
	if n.NetworkID != "" {
		relayedClientsQ = relayedClientsQ.Where("network_id = ?", n.NetworkID)
		relayedIGWQ = relayedIGWQ.Where("network_id = ?", n.NetworkID)
	}

	err = relayedClientsQ.UpdateColumn("relayed_clients", expr.Remove("relayed_clients", n.ID)).Error
	if err != nil {
		return err
	}

	return relayedIGWQ.UpdateColumn("relayed_igw_clients", expr.Remove("relayed_igw_clients", n.ID)).Error
}

func (n *Node) ResetGateway(ctx context.Context) error {
	err := db.FromContext(ctx).Model(&Node{}).
		Where("id = ?", n.ID).
		Updates(map[string]interface{}{
			"is_gateway":                   n.IsGateway,
			"is_internet_gateway":          n.IsInternetGateway,
			"is_auto_relay":                n.IsAutoRelay,
			"relayed_clients":              n.RelayedClients,
			"relayed_igw_clients":          n.RelayedIGWClients,
			"additional_gateway_endpoints": n.AdditionalGatewayEndpoints,
		}).Error
	if err != nil {
		return err
	}

	// Only unassign clients relayed by THIS gateway. Clearing the whole network
	// would drop RelayedBy / IsIGWClient for exit clients of other exit nodes.
	err = db.FromContext(ctx).Model(&Node{}).
		Where("network_id = ?", n.NetworkID).
		Where("relayed_by_node_id = ?", n.ID).
		Updates(map[string]interface{}{
			"relayed_by_node_id": nil,
			"is_igw_client":      false,
			"use_tcp_uplink":     false,
		}).Error
	if err != nil {
		return err
	}

	return db.FromContext(ctx).Model(&Node{}).
		Where("network_id = ?", n.NetworkID).
		Where(expr.WhereHasValue("auto_relayed_peers", n.ID)).
		UpdateColumn("auto_relayed_peers", expr.RemoveByValue("auto_relayed_peers", n.ID)).
		Error
}

func (n *Node) ClearGatewayIDFromEnrollmentKeys(ctx context.Context) error {
	return db.FromContext(ctx).Model(&EnrollmentKey{}).
		Where("gateway_id = ?", n.ID).
		Update("gateway_id", nil).
		Error
}
