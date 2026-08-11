package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"golang.org/x/exp/slog"
	"gorm.io/datatypes"
)

var ValidateEgressReq = validateEgressReq

var ErrEgressLimitExceeded = errors.New("egress limit reached for this tenant, please upgrade your license")

var EgressLimitExceeded = func(ctx context.Context) bool {
	return false
}

var AssignVirtualRangeToEgress = func(nw *schema.Network, eg *schema.Egress) error {
	return nil
}

func validateEgressReq(ctx context.Context, e *schema.Egress) error {
	if e.Network == "" {
		return errors.New("network id is empty")
	}
	NormalizeEgressType(e)
	if err := ValidateEgressAppNATMode(*e); err != nil {
		return err
	}
	if err := ValidateEgressProOnlyFeatures(*e); err != nil {
		return err
	}
	if IsEgressInternetGateway(*e) {
		e.Type = schema.EgressTypeInternet
		e.Range = "*"
		e.Domains = nil
		e.PresetID = ""
		e.VirtualRange = ""
		if e.Nat {
			e.Mode = schema.DirectNAT
		} else {
			e.Mode = schema.DisabledNAT
		}
	} else if e.Nat {
		e.Mode = schema.DirectNAT
	} else {
		e.Mode = schema.DisabledNAT
		e.VirtualRange = ""
	}
	network := &schema.Network{Name: e.Network}
	if err := network.Get(ctx); err != nil {
		return errors.New("failed to get network " + err.Error())
	}
	if e.Range != "" {
		if err := ValidateEgressCIDR(network, e.Range); err != nil {
			return err
		}
	}
	if !GetFeatureFlags(ctx).EnableEgressHA && len(e.Nodes) > 1 {
		return errors.New("can only set one routing node on CE")
	}

	if len(e.Nodes) > 0 {
		for k := range e.Nodes {
			node, err := GetNodeByID(k)
			if err != nil {
				return errors.New("invalid routing node " + err.Error())
			}
			if IsEgressInternetGateway(*e) {
				if err := ValidateInternetEgressRoutingNode(&node); err != nil {
					return err
				}
			}
		}
	}
	if IsEgressInternetGateway(*e) && len(e.Tags) > 0 {
		return errors.New("internet egress must use explicit routing nodes, not tags")
	}
	return nil
}

// NormalizeEgressReqDomains validates each domain entry (FQDN or *.suffix),
// lowercases, and deduplicates while preserving input order.
func NormalizeEgressReqDomains(domains []string) ([]string, error) {
	seen := make(map[string]struct{})
	var out []string
	add := func(s string) error {
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "" {
			return nil
		}
		if !IsEgressDomainPattern(s) {
			return fmt.Errorf("invalid egress domain: %q", s)
		}
		if _, ok := seen[s]; ok {
			return nil
		}
		seen[s] = struct{}{}
		out = append(out, s)
		return nil
	}
	for _, d := range domains {
		if err := add(d); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// ConfiguredDomainsForEgress returns the user-configured hostname list from e.Domains (JSON).
// It does not read the legacy DB column "domain" (singular); that is migrated once in migrateEgressDomains.
func ConfiguredDomainsForEgress(e schema.Egress) []string {
	if len(e.Domains) == 0 {
		return nil
	}
	out := make([]string, len(e.Domains))
	copy(out, e.Domains)
	return out
}

// ApplyConfiguredDomainsToEgress sets Domains on the egress record.
func ApplyConfiguredDomainsToEgress(e *schema.Egress, domains []string) {
	e.Domains = datatypes.JSONSlice[string](domains)
}

// IsDomainBasedEgress is true when this egress has at least one configured logical domain.
func IsDomainBasedEgress(e schema.Egress) bool {
	return len(ConfiguredDomainsForEgress(e)) > 0
}

// IsEgressInternetGateway is true when type is internet or range is "*" (full internet egress).
func IsEgressInternetGateway(e schema.Egress) bool {
	if e.Type == schema.EgressTypeInternet {
		return true
	}
	return strings.TrimSpace(e.Range) == "*"
}

// IsEgressReqInternetGateway is true when the request uses type internet or range "*" for internet egress.
func IsEgressReqInternetGateway(req *models.EgressReq) bool {
	if req == nil {
		return false
	}
	if req.Type == schema.EgressTypeInternet {
		return true
	}
	return strings.TrimSpace(req.Range) == "*"
}

// InferEgressType derives the egress type from request fields when Type is unset.
func InferEgressType(req *models.EgressReq) schema.EgressType {
	if req == nil {
		return schema.EgressTypeCIDR
	}
	if req.Type != "" {
		return req.Type
	}
	if strings.TrimSpace(req.Range) == "*" {
		return schema.EgressTypeInternet
	}
	if strings.TrimSpace(req.PresetID) != "" {
		return schema.EgressTypeApp
	}
	if len(req.Domains) > 0 {
		return schema.EgressTypeDomain
	}
	return schema.EgressTypeCIDR
}

// NormalizeEgressType sets Type from range/domains/preset when empty, and forces internet invariants.
func NormalizeEgressType(e *schema.Egress) {
	if e == nil {
		return
	}
	if e.Type == "" {
		switch {
		case strings.TrimSpace(e.Range) == "*":
			e.Type = schema.EgressTypeInternet
		case strings.TrimSpace(e.PresetID) != "":
			e.Type = schema.EgressTypeApp
		case len(ConfiguredDomainsForEgress(*e)) > 0:
			e.Type = schema.EgressTypeDomain
		default:
			e.Type = schema.EgressTypeCIDR
		}
	}
	if e.Type == schema.EgressTypeInternet {
		e.Range = "*"
		e.Domains = nil
		e.PresetID = ""
		e.VirtualRange = ""
	}
}

// InternetEgressRanges returns the WireGuard/firewall ranges for an internet egress.
func InternetEgressRanges(includeIPv6 bool) []string {
	ranges := []string{IPv4Network}
	if includeIPv6 {
		ranges = append(ranges, IPv6Network)
	}
	return ranges
}

// ExpandEgressRouteRanges maps an egress resource to concrete CIDR ranges for peer/firewall config.
// Internet egress expands "*" to 0.0.0.0/0 (and optionally ::/0).
func ExpandEgressRouteRanges(e schema.Egress, includeIPv6 bool) []string {
	if IsEgressInternetGateway(e) {
		return InternetEgressRanges(includeIPv6)
	}
	if e.Range != "" {
		egressRange := e.Range
		if e.Nat && e.VirtualRange != "" {
			egressRange = e.VirtualRange
		}
		return []string{egressRange}
	}
	return AllDomainAnsFromEgress(e)
}

// ValidateInternetEgressRoutingNode ensures a routing node can act as an internet exit node.
func ValidateInternetEgressRoutingNode(node *models.Node) error {
	if node == nil {
		return errors.New("routing node is required")
	}
	host := &schema.Host{ID: node.HostID}
	if err := host.Get(db.WithContext(context.TODO())); err != nil {
		return err
	}
	if host.OS != models.OS_Types.Linux {
		return errors.New("only linux nodes can be internet egress routing nodes")
	}
	if host.FirewallInUse == schema.FIREWALL_NONE {
		return errors.New("iptables or nftables needs to be installed")
	}
	if InternetExitRoutingNodeID(node) != "" {
		return fmt.Errorf("node %s is using an internet gateway already", host.Name)
	}
	if node.IsRelayed {
		return fmt.Errorf("node %s is being relayed", host.Name)
	}
	return nil
}

// ErrExitNodeBlocksGatewayOps returns an error when the node must not use GW assign/unassign/auto-assign.
// Exit clients have RelayedBy managed by exit selection; exit routers cannot be gateway clients.
func ErrExitNodeBlocksGatewayOps(node *models.Node) error {
	if node == nil {
		return nil
	}
	if node.SelectedInternetEgressID != "" {
		return errors.New("node is using an exit node; gateway assignment is managed by the exit node")
	}
	if NodeIsInternetEgressRouter(node.ID.String(), node.Network) {
		return errors.New("exit node cannot use gateway or auto-assign gateway options")
	}
	return nil
}

// ErrExitNodeBlocksAutoRelay returns an error when the node must not be auto-relayed
// (as victim/peer). Exit clients already have RelayedBy managed by exit selection;
// exit routing nodes must not be auto-relayed (same as manual relay). Exit routing
// nodes may still act as auto-relay gateways for other peers.
func ErrExitNodeBlocksAutoRelay(node *models.Node) error {
	if node == nil {
		return nil
	}
	if node.SelectedInternetEgressID != "" {
		return errors.New("node is using an exit node; auto-relay is not allowed")
	}
	if IsInternetGw(*node) {
		return errors.New("exit node cannot be auto-relayed")
	}
	return nil
}

// ErrExitClientBlocksAutoRelayRole returns an error when an exit client tries to act
// as an auto-relay gateway. Exit routing nodes remain valid auto-relay targets.
func ErrExitClientBlocksAutoRelayRole(node *models.Node) error {
	if node == nil {
		return nil
	}
	if node.SelectedInternetEgressID != "" {
		return errors.New("node is using an exit node; auto-relay is not allowed")
	}
	return nil
}

// ClearExitNodeForDisconnect clears the node's internet exit selection so a
// subsequent peer update can remove full-tunnel routes before disconnect.
// Returns true when a selection was cleared. Callers should sync related fields
// onto any in-flight node copy (SelectedInternetEgressID, InternetGwID,
// IsRelayed, RelayedBy) from the updated node.
func ClearExitNodeForDisconnect(node *models.Node) (bool, error) {
	if node == nil || node.SelectedInternetEgressID == "" {
		return false, nil
	}
	if err := SetNodeSelectedInternetEgress(node, ""); err != nil {
		return false, err
	}
	return true, nil
}

// SyncClearedExitNodeFields copies exit/relay fields after ClearExitNodeForDisconnect
// onto another node object that will be persisted (e.g. the disconnect update payload).
func SyncClearedExitNodeFields(dst, src *models.Node) {
	if dst == nil || src == nil {
		return
	}
	dst.SelectedInternetEgressID = src.SelectedInternetEgressID
	dst.InternetGwID = src.InternetGwID
	dst.IsRelayed = src.IsRelayed
	dst.RelayedBy = src.RelayedBy
	dst.UseTcpUplink = src.UseTcpUplink
}

// NodeIsInternetEgressRouter reports whether the node is a routing node for any active internet egress.
func NodeIsInternetEgressRouter(nodeID, network string) bool {
	if nodeID == "" || network == "" {
		return false
	}
	eli, err := (&schema.Egress{Network: network}).ListByNetwork(db.WithContext(context.TODO()))
	if err != nil {
		return false
	}
	for _, e := range eli {
		if !e.Status || !IsEgressInternetGateway(e) {
			continue
		}
		if _, ok := e.Nodes[nodeID]; ok {
			return true
		}
	}
	return false
}

// GetSelectedInternetEgress returns the internet egress selected by the node, if any and still valid.
func GetSelectedInternetEgress(node *models.Node) (*schema.Egress, error) {
	if node == nil || node.SelectedInternetEgressID == "" {
		return nil, errors.New("no internet egress selected")
	}
	e := &schema.Egress{ID: node.SelectedInternetEgressID}
	if err := e.Get(db.WithContext(context.TODO())); err != nil {
		return nil, err
	}
	if !e.Status || e.Network != node.Network || !IsEgressInternetGateway(*e) {
		return nil, errors.New("selected internet egress is not available")
	}
	return e, nil
}

// InternetExitRoutingNodeID returns the node ID used for full-internet exit.
// Prefers SelectedInternetEgressID (source of truth). When a selection is set but the
// egress is unavailable (disabled/missing), returns "" so clients fail open to local
// internet while keeping the sticky selection — do not fall back to InternetGwID in
// that case. Legacy InternetGwID is only used when no selection is set.
func InternetExitRoutingNodeID(node *models.Node) string {
	if node == nil {
		return ""
	}
	if node.SelectedInternetEgressID != "" {
		if e, err := GetSelectedInternetEgress(node); err == nil {
			if id := FirstInternetEgressRoutingNodeID(*e); id != "" {
				return id
			}
		}
		// Selection present but egress unavailable: fail open (sticky selection).
		return ""
	}
	return node.InternetGwID
}

// ResolveInternetExitRoutingNode sets node.InternetGwID in-memory from SelectedInternetEgressID
// for peer-update helpers that still read InternetGwID. Does not persist.
// When the selected egress is unavailable, clears InternetGwID so fail-open is consistent.
func ResolveInternetExitRoutingNode(node *models.Node) {
	if node == nil {
		return
	}
	if id := InternetExitRoutingNodeID(node); id != "" {
		node.InternetGwID = id
		return
	}
	if node.SelectedInternetEgressID != "" {
		// Sticky selection but egress unavailable (disabled/missing): fail open.
		node.InternetGwID = ""
	}
}

// FirstInternetEgressRoutingNodeID returns a routing node ID from an internet egress.
func FirstInternetEgressRoutingNodeID(e schema.Egress) string {
	for nodeID := range e.Nodes {
		if nodeID != "" {
			return nodeID
		}
	}
	return ""
}

// CreateInternetEgressForNode creates an internet-type egress with the given node as routing node.
func CreateInternetEgressForNode(ctx context.Context, node *models.Node, name, createdBy string) (*schema.Egress, error) {
	if node == nil {
		return nil, errors.New("routing node is required")
	}
	if err := ValidateInternetEgressRoutingNode(node); err != nil {
		return nil, err
	}
	if name == "" {
		host := &schema.Host{ID: node.HostID}
		_ = host.Get(ctx)
		if host.Name != "" {
			name = host.Name + "-internet"
		} else {
			name = node.ID.String() + "-internet"
		}
	}
	e := &schema.Egress{
		ID:        uuid.New().String(),
		TenantID:  scope.ID(ctx),
		Name:      name,
		Network:   node.Network,
		Type:      schema.EgressTypeInternet,
		Range:     "*",
		Nat:       true,
		Mode:      schema.DirectNAT,
		Nodes:     datatypes.JSONMap{node.ID.String(): 256},
		Tags:      make(datatypes.JSONMap),
		Status:    true,
		CreatedBy: createdBy,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := ValidateEgressReq(ctx, e); err != nil {
		return nil, err
	}
	if e.TenantID == "" {
		// tenant may be set by caller/middleware; leave empty if unset
	}
	if err := e.Create(ctx); err != nil {
		return nil, err
	}
	return e, nil
}

// FindInternetEgressByRoutingNode returns an active internet egress that uses nodeID as a routing node.
func FindInternetEgressByRoutingNode(ctx context.Context, network, nodeID string) (*schema.Egress, error) {
	eli, err := (&schema.Egress{Network: network}).ListByNetwork(db.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	for i := range eli {
		e := eli[i]
		if !IsEgressInternetGateway(e) {
			continue
		}
		if _, ok := e.Nodes[nodeID]; ok {
			return &e, nil
		}
	}
	return nil, errors.New("internet egress not found for routing node")
}

// SetNodeSelectedInternetEgress sets or clears the node's selected internet egress.
// Selecting an exit also relays the node through that egress routing node (IsIGWClient),
// so NAT'd full-tunnel clients remain reachable via the gateway. InternetGwID is not dual-written.
func SetNodeSelectedInternetEgress(node *models.Node, egressID string) error {
	if node == nil {
		return errors.New("node is required")
	}
	ctx := scope.WithContext(db.WithContext(context.TODO()), scope.TenantScope, node.TenantID)
	schemaNode := &schema.Node{ID: node.ID.String()}
	if err := schemaNode.Get(ctx); err != nil {
		return err
	}
	if schemaNode.NetworkID == "" && node.Network != "" {
		nw := &schema.Network{Name: node.Network}
		if err := nw.Get(ctx); err == nil {
			schemaNode.NetworkID = nw.ID
		}
	}

	if egressID == "" {
		return clearNodeSelectedInternetEgress(ctx, node, schemaNode)
	}

	e := &schema.Egress{ID: egressID}
	if err := e.Get(ctx); err != nil {
		return err
	}
	if !e.Status || e.Network != node.Network || !IsEgressInternetGateway(*e) {
		return errors.New("egress is not an active internet exit node in this network")
	}
	routingNodeID := FirstInternetEgressRoutingNodeID(*e)
	if routingNodeID == "" {
		return errors.New("internet egress has no routing node")
	}
	if err := validateInternetEgressSelection(
		node,
		schemaNode,
		routingNodeID,
		NodeIsInternetEgressRouter(node.ID.String(), node.Network),
	); err != nil {
		return err
	}

	if schemaNode.AutoAssignGateway {
		schemaNode.AutoAssignGateway = false
		if err := schemaNode.ResetAutoAssignGateway(ctx); err != nil {
			return err
		}
	}
	if len(schemaNode.AutoRelayedPeers.Data()) > 0 {
		_ = schemaNode.ResetAutoRelayedPeers(ctx)
	}

	// Phase 1: relay the node through the exit routing node without enabling full
	// tunnel yet (IsIGWClient stays false so InternetGwID/exit resolution does not
	// kick in). Publish a peer update so clients establish mesh reachability via the
	// gateway before the exit (0.0.0.0/0) config is applied.
	alreadyRelayedByRoutingNode := schemaNode.RelayedByNodeID != nil &&
		*schemaNode.RelayedByNodeID == routingNodeID
	if !alreadyRelayedByRoutingNode {
		schemaNode.IsIGWClient = false
		schemaNode.RelayedByNodeID = &routingNodeID
		if err := schemaNode.AssignGateway(ctx); err != nil {
			return err
		}
		PublishPeerUpdateAfterExitNodeChange(ctx)
		time.Sleep(1 * time.Second)
	}

	// Phase 2: apply the exit selection and mark the node as an IGW client so full
	// tunnel resolves. Callers publish the second (exit) peer update after return.
	schemaNode.IsIGWClient = true
	schemaNode.RelayedByNodeID = &routingNodeID
	schemaNode.SelectedInternetEgressID = egressID
	if err := schemaNode.AssignGateway(ctx); err != nil {
		return err
	}
	if err := db.FromContext(ctx).Model(&schema.Node{}).Where("id = ?", schemaNode.ID).
		Update("selected_internet_egress_id", egressID).Error; err != nil {
		return err
	}

	updated := ConvertSchemaNodeToModelsNode(schemaNode)
	if updated == nil || updated.ID.String() == "" {
		return errors.New("failed to reload node after exit node selection")
	}
	updated.SelectedInternetEgressID = egressID
	updated.InternetGwID = ""
	*node = *updated
	return nil
}

func validateInternetEgressSelection(
	node *models.Node,
	schemaNode *schema.Node,
	routingNodeID string,
	isExitNode bool,
) error {
	if isExitNode {
		return errors.New("exit node cannot use another exit node")
	}
	if routingNodeID == node.ID.String() {
		return errors.New("routing node cannot select itself as exit node")
	}
	if node.IsGw || node.IsIngressGateway || node.IsRelay || node.IsInternetGateway || schemaNode.IsGateway {
		return errors.New("gateway nodes cannot be assigned an exit node")
	}
	if schemaNode.RelayedByNodeID != nil && *schemaNode.RelayedByNodeID != "" && *schemaNode.RelayedByNodeID != routingNodeID {
		return errors.New("node is relayed by a different gateway")
	}
	return nil
}

func clearNodeSelectedInternetEgress(ctx context.Context, node *models.Node, schemaNode *schema.Node) error {
	if err := db.FromContext(ctx).Model(&schema.Node{}).Where("id = ?", schemaNode.ID).
		Update("selected_internet_egress_id", "").Error; err != nil {
		return err
	}
	schemaNode.SelectedInternetEgressID = ""

	// Always drop exit RelayedBy / IsIGWClient when clearing selection. RelayedBy may
	// still point at a deleted routing node while egress.Nodes already lists a new one;
	// matching only FirstInternetEgressRoutingNodeID would leave a stale RelayedBy and
	// block reassignment with "relayed by a different gateway".
	schemaNode.RelayedByNodeID = nil
	schemaNode.IsIGWClient = false
	if err := schemaNode.UnassignGateway(ctx); err != nil {
		return err
	}

	updated := ConvertSchemaNodeToModelsNode(schemaNode)
	if updated == nil || updated.ID.String() == "" {
		return errors.New("failed to reload node after clearing exit node")
	}
	updated.SelectedInternetEgressID = ""
	updated.InternetGwID = ""
	*node = *updated
	return nil
}

// failOpenExitClientKeepSelection clears RelayedBy / IsIGWClient so full-tunnel
// routes fail open, while keeping SelectedInternetEgressID for sticky reassignment.
func failOpenExitClientKeepSelection(ctx context.Context, node *models.Node) error {
	if node == nil {
		return errors.New("node is required")
	}
	dbCtx := db.WithContext(ctx)
	schemaNode := &schema.Node{ID: node.ID.String()}
	if err := schemaNode.Get(dbCtx); err != nil {
		return err
	}
	if schemaNode.NetworkID == "" && node.Network != "" {
		nw := &schema.Network{Name: node.Network}
		if err := nw.Get(dbCtx); err == nil {
			schemaNode.NetworkID = nw.ID
		}
	}
	if !schemaNode.IsIGWClient &&
		(schemaNode.RelayedByNodeID == nil || *schemaNode.RelayedByNodeID == "") {
		return nil
	}
	schemaNode.RelayedByNodeID = nil
	schemaNode.IsIGWClient = false
	if err := schemaNode.UnassignGateway(dbCtx); err != nil {
		return err
	}
	updated := ConvertSchemaNodeToModelsNode(schemaNode)
	if updated == nil || updated.ID.String() == "" {
		return errors.New("failed to reload node after exit fail-open")
	}
	// Preserve sticky selection from the pre-update node / DB column.
	updated.SelectedInternetEgressID = node.SelectedInternetEgressID
	if updated.SelectedInternetEgressID == "" {
		updated.SelectedInternetEgressID = schemaNode.SelectedInternetEgressID
	}
	updated.InternetGwID = ""
	*node = *updated
	return nil
}

// ClearNodesSelectedInternetEgress clears SelectedInternetEgressID for all nodes that selected this egress.
func ClearNodesSelectedInternetEgress(ctx context.Context, egressID, network string) {
	if egressID == "" || network == "" {
		return
	}
	nodes, err := GetNetworkNodes(ctx, network)
	if err != nil {
		return
	}
	for _, node := range nodes {
		if node.SelectedInternetEgressID == egressID {
			n := node
			_ = SetNodeSelectedInternetEgress(&n, "")
		}
	}
}

// ListNodesBySelectedInternetEgress returns network nodes that have selected the given egress.
func ListNodesBySelectedInternetEgress(ctx context.Context, network, egressID string) []models.Node {
	if network == "" || egressID == "" {
		return nil
	}
	nodes, err := GetNetworkNodes(ctx, network)
	if err != nil {
		return nil
	}
	out := make([]models.Node, 0)
	for _, node := range nodes {
		if node.SelectedInternetEgressID == egressID {
			out = append(out, node)
		}
	}
	return out
}

// ListExitClientsForRoutingNode returns mesh nodes using this node as their internet exit,
// via RelayedIGWClients and/or SelectedInternetEgressID pointing at an egress this node routes.
func ListExitClientsForRoutingNode(ctx context.Context, network, routingNodeID string) []models.Node {
	if network == "" || routingNodeID == "" {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]models.Node, 0)

	addClient := func(id string) {
		if id == "" || id == routingNodeID {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		n, err := GetNodeByID(id)
		if err != nil {
			return
		}
		seen[id] = struct{}{}
		out = append(out, n)
	}

	routing := &schema.Node{ID: routingNodeID}
	if err := routing.Get(ctx); err == nil {
		for clientID := range routing.RelayedIGWClients {
			addClient(clientID)
		}
	}

	if e, err := FindInternetEgressByRoutingNode(ctx, network, routingNodeID); err == nil && e != nil {
		for _, n := range ListNodesBySelectedInternetEgress(ctx, network, e.ID) {
			if _, ok := seen[n.ID.String()]; ok {
				continue
			}
			seen[n.ID.String()] = struct{}{}
			out = append(out, n)
		}
	}
	return out
}

// DeleteInternetEgressesForRoutingNode removes internet egress resources where nodeID is a routing node.
func DeleteInternetEgressesForRoutingNode(ctx context.Context, network, nodeID string) {
	eli, err := (&schema.Egress{Network: network}).ListByNetwork(ctx)
	if err != nil {
		return
	}
	for _, e := range eli {
		if !IsEgressInternetGateway(e) {
			continue
		}
		if _, ok := e.Nodes[nodeID]; !ok {
			continue
		}
		ClearNodesSelectedInternetEgress(ctx, e.ID, network)
		_ = e.Delete(ctx)
	}
}

// PublishExitClientsFailOpen is wired from mq to push fail-open peer updates to
// exit clients before their routing node is removed from the mesh.
var PublishExitClientsFailOpen = func(ctx context.Context, clients []models.Node) {}

// FailOpenAndDetachExitRoutingNode detaches exit routing state for a node about to
// be removed, then pushes ordered fail-open peer updates to affected clients.
func FailOpenAndDetachExitRoutingNode(ctx context.Context, node *models.Node) {
	clients := DetachExitRoutingNode(ctx, node)
	if len(clients) == 0 {
		return
	}
	PublishExitClientsFailOpen(ctx, clients)
	// Give clients a moment to apply fail-open before the peer disappears.
	time.Sleep(1 * time.Second)
}

// DetachExitRoutingNode prepares a routing node for removal from the network:
// fails open RelayedBy / IsIGWClient for exit clients (keeps sticky
// SelectedInternetEgressID), removes the node from internet egress routing maps
// (egress resource is kept for reassignment), and returns affected clients for
// an ordered fail-open peer update.
func DetachExitRoutingNode(ctx context.Context, node *models.Node) []models.Node {
	if node == nil || node.Network == "" || node.ID == uuid.Nil {
		return nil
	}
	// List clients before mutating egress.Nodes so sticky selections are still found.
	clients := ListExitClientsForRoutingNode(ctx, node.Network, node.ID.String())
	if len(clients) == 0 && !IsInternetGw(*node) && !NodeIsInternetEgressRouter(node.ID.String(), node.Network) {
		return nil
	}

	dbCtx := db.WithContext(ctx)
	eli, err := (&schema.Egress{Network: node.Network}).ListByNetwork(dbCtx)
	if err != nil {
		slog.Error("DetachExitRoutingNode: list egresses failed", "error", err)
	}
	for i := range eli {
		e := &eli[i]
		if !IsEgressInternetGateway(*e) {
			continue
		}
		if _, ok := e.Nodes[node.ID.String()]; !ok {
			continue
		}
		// Keep egress + sticky client selections; empty Nodes → fail-open exit routes.
		delete(e.Nodes, node.ID.String())
		newNodes := make(datatypes.JSONMap)
		for k, v := range e.Nodes {
			newNodes[k] = v
		}
		if err := db.FromContext(dbCtx).Table(e.Table()).Where("id = ?", e.ID).Updates(map[string]any{
			"nodes": newNodes,
		}).Error; err != nil {
			slog.Error("DetachExitRoutingNode: failed to update egress nodes", "id", e.ID, "error", err)
		}
	}

	for i := range clients {
		c := &clients[i]
		if err := failOpenExitClientKeepSelection(ctx, c); err != nil {
			slog.Error("DetachExitRoutingNode: fail-open client failed", "node", c.ID, "error", err)
		}
		if c.InternetGwID == node.ID.String() {
			c.InternetGwID = ""
			_ = UpsertNode(c)
		}
	}
	if node.IsInternetGateway {
		UnsetInternetGw(ctx, node)
	}
	return clients
}

// RebindInternetEgressClients re-applies exit selection for clients of an internet
// egress after its routing node set changes (e.g. reassignment to a new node).
func RebindInternetEgressClients(ctx context.Context, e schema.Egress) {
	if !IsEgressInternetGateway(e) {
		return
	}
	clients := ListNodesBySelectedInternetEgress(ctx, e.Network, e.ID)
	routingID := FirstInternetEgressRoutingNodeID(e)
	for i := range clients {
		c := clients[i]
		// Drop RelayedBy to a removed/old routing node first; otherwise
		// SetNodeSelectedInternetEgress rejects "relayed by a different gateway".
		if err := failOpenExitClientKeepSelection(ctx, &c); err != nil {
			slog.Error("RebindInternetEgressClients: fail-open failed", "node", c.ID, "error", err)
			continue
		}
		if routingID == "" || !e.Status {
			continue
		}
		if err := SetNodeSelectedInternetEgress(&c, e.ID); err != nil {
			slog.Error("RebindInternetEgressClients: rebind failed", "node", c.ID, "egress", e.ID, "error", err)
		}
	}
}

// EgressDomainsEqual compares two domain lists as sets (order-independent).
func EgressDomainsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aa := slices.Clone(a)
	bb := slices.Clone(b)
	slices.Sort(aa)
	slices.Sort(bb)
	return slices.Equal(aa, bb)
}

func DoesUserHaveAccessToEgress(user *schema.User, e *schema.Egress, acls []models.Acl) bool {
	if !e.Status {
		return false
	}
	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		dstTags := ConvAclTagToValueMap(acl.Dst)
		_, all := dstTags["*"]

		if _, ok := dstTags[e.ID]; ok || all {
			// get all src tags
			for _, srcAcl := range acl.Src {
				if srcAcl.ID == models.UserAclID && srcAcl.Value == user.Username {
					return true
				} else if srcAcl.ID == models.UserGroupAclID {
					// fetch all users in the group
					if _, ok := user.UserGroups.Data()[schema.UserGroupID(srcAcl.Value)]; ok {
						return true
					}
				}
			}
		}
	}
	return false
}

func DoesNodeHaveAccessToEgress(node *models.Node, e *schema.Egress, acls []models.Acl) bool {
	nodeTags := maps.Clone(node.Tags)
	nodeTags[models.TagID(node.ID.String())] = struct{}{}
	nodeTags[models.TagID("*")] = struct{}{}
	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		srcVal := ConvAclTagToValueMap(acl.Src)
		for _, dstI := range acl.Dst {
			if (dstI.ID == models.EgressID && dstI.Value == e.ID) || (dstI.ID == models.NodeTagID && dstI.Value == "*") {
				if dstI.ID == models.EgressID {
					e := schema.Egress{ID: dstI.Value}
					err := e.Get(db.WithContext(context.TODO()))
					if err != nil {
						continue
					}
				}

				if node.IsStatic {
					if _, ok := srcVal[node.StaticNode.ClientID]; ok {
						return true
					}
				} else {
					if _, ok := srcVal[node.ID.String()]; ok {
						return true
					}
				}

				for tagID := range nodeTags {
					if _, ok := srcVal[tagID.String()]; ok {
						return true
					}
				}
			}

		}
	}
	return false
}

func doesNodeHaveAccessToEgressByRoutingPolicy(node, targetNode *models.Node, e *schema.Egress, acls []models.Acl) bool {
	if node == nil || targetNode == nil || e == nil {
		return false
	}
	if _, ok := e.Nodes[targetNode.ID.String()]; !ok {
		return false
	}
	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		if !IsEgressRoutingPolicyAllowedForNodes(acl, *node, *targetNode) {
			continue
		}
		srcEgresses := getEgressesFromPolicyTags(acl.Src, node.Network)
		dstEgresses := getEgressesFromPolicyTags(acl.Dst, node.Network)
		nodeRoutesSrc := targetNodeRoutesAnyEgress(*node, srcEgresses)
		nodeRoutesDst := targetNodeRoutesAnyEgress(*node, dstEgresses)
		targetRoutesSrc := targetNodeRoutesAnyEgress(*targetNode, srcEgresses)
		targetRoutesDst := targetNodeRoutesAnyEgress(*targetNode, dstEgresses)
		if acl.AllowedDirection == models.TrafficDirectionUni {
			if nodeRoutesSrc && targetRoutesDst && egressListContainsID(dstEgresses, e.ID) {
				return true
			}
			continue
		}
		if nodeRoutesSrc && targetRoutesDst && egressListContainsID(dstEgresses, e.ID) {
			return true
		}
		if nodeRoutesDst && targetRoutesSrc && egressListContainsID(srcEgresses, e.ID) {
			return true
		}
	}
	return false
}

func egressListContainsID(egresses []schema.Egress, id string) bool {
	for _, e := range egresses {
		if e.ID == id {
			return true
		}
	}
	return false
}

// snapshotNodeTagIDs copies tag keys from n.Tags. When n.Mutex is set, reads are serialized
// with writers on the same node (shallow copies may share the Tags map). When Mutex is nil,
// tags are still read so tag-based egress matching applies; that matches patterns like
// maps.Clone(node.Tags) elsewhere for nodes without an initialized mutex.
func snapshotNodeTagIDs(n *models.Node) []models.TagID {
	if n == nil {
		return nil
	}
	if n.Mutex != nil {
		n.Mutex.Lock()
		defer n.Mutex.Unlock()
	}
	if len(n.Tags) == 0 {
		return nil
	}
	out := make([]models.TagID, 0, len(n.Tags))
	for tid := range n.Tags {
		out = append(out, tid)
	}
	return out
}

func appendEgressRangesToReq(req *models.EgressGatewayRequest, e schema.Egress, metric uint32, includeIPv6 bool) {
	if req == nil {
		return
	}
	if IsEgressInternetGateway(e) {
		ranges := ExpandEgressRouteRanges(e, includeIPv6)
		req.Ranges = append(req.Ranges, ranges...)
		for _, rangeI := range ranges {
			req.RangesWithMetric = append(req.RangesWithMetric, models.EgressRangeMetric{
				EgressID:    e.ID,
				EgressName:  e.Name,
				Network:     rangeI,
				Nat:         e.Nat,
				Mode:        e.Mode,
				RouteMetric: metric,
			})
		}
		return
	}
	if e.Range != "" {
		egressRange := e.Range
		if e.Nat && e.VirtualRange != "" {
			egressRange = e.VirtualRange
		}
		req.Ranges = append(req.Ranges, egressRange)
		req.RangesWithMetric = append(req.RangesWithMetric, models.EgressRangeMetric{
			EgressID:       e.ID,
			EgressName:     e.Name,
			Network:        e.Range,
			VirtualNetwork: e.VirtualRange,
			Nat:            e.Nat,
			Mode:           e.Mode,
			RouteMetric:    metric,
		})
	}
	if IsDomainBasedEgress(e) && HasEgressDomainAns(e) {
		req.Ranges = append(req.Ranges, AllDomainAnsFromEgress(e)...)
		for _, domainAnsI := range AllDomainAnsFromEgress(e) {
			req.RangesWithMetric = append(req.RangesWithMetric, models.EgressRangeMetric{
				EgressID:       e.ID,
				EgressName:     e.Name,
				Network:        domainAnsI,
				VirtualNetwork: e.VirtualRange,
				Nat:            e.Nat,
				Mode:           e.Mode,
				RouteMetric:    metric,
			})
		}
	} else if e.Range == "" {
		req.Ranges = append(req.Ranges, AllDomainAnsFromEgress(e)...)
	}
}

func AddEgressInfoToPeerByAccess(node, targetNode *models.Node, eli []schema.Egress, acls []models.Acl, isDefaultPolicyActive bool) {

	req := models.EgressGatewayRequest{
		NodeID:     targetNode.ID.String(),
		NetID:      targetNode.Network,
		NatEnabled: "yes",
	}
	nodeTagIDs := snapshotNodeTagIDs(targetNode)
	// For internet egress, ::/0 is gated on the exit host's public IPv6 endpoint.
	includeIPv6 := exitHostHasEndpointIPv6(targetNode)
	for _, e := range eli {
		if !e.Status || e.Network != targetNode.Network {
			continue
		}
		if IsEgressInternetGateway(e) && !usesPeerAsInternetExit(node, targetNode) {
			continue
		}
		if !isDefaultPolicyActive {
			if !DoesNodeHaveAccessToEgress(node, &e, acls) &&
				!doesNodeHaveAccessToEgressByRoutingPolicy(node, targetNode, &e, acls) {
				if node.IsRelayed && node.RelayedBy == targetNode.ID.String() {
					if !DoesNodeHaveAccessToEgress(targetNode, &e, acls) {
						continue
					}
				} else {
					continue
				}

			}
		}

		if metric, ok := e.Nodes[targetNode.ID.String()]; ok {
			m64, err := metric.(json.Number).Int64()
			if err != nil {
				m64 = 256
			}
			appendEgressRangesToReq(&req, e, uint32(m64), includeIPv6)
		}
		for _, tagID := range nodeTagIDs {
			if metric, ok := e.Tags[tagID.String()]; ok {
				m64, err := metric.(json.Number).Int64()
				if err != nil {
					m64 = 256
				}
				appendEgressRangesToReq(&req, e, uint32(m64), includeIPv6)
				break
			}
		}

	}
	if targetNode.Mutex != nil {
		targetNode.Mutex.Lock()
	}
	if len(req.Ranges) > 0 {

		targetNode.EgressDetails.IsEgressGateway = true
		targetNode.EgressDetails.EgressGatewayRanges = req.Ranges
		targetNode.EgressDetails.EgressGatewayRequest = req

	} else {
		targetNode.EgressDetails = models.EgressDetails{}
	}
	if targetNode.Mutex != nil {
		targetNode.Mutex.Unlock()
	}
}

func GetEgressDomainsByAccessForUser(ctx context.Context, user *schema.User, network schema.NetworkID) (domains []string) {
	acls := ListUserPolicies(ctx, network)
	eli, _ := (&schema.Egress{Network: network.String()}).ListByNetwork(ctx)
	defaultDevicePolicy, _ := GetDefaultPolicy(ctx, network, models.UserPolicy)
	isDefaultPolicyActive := defaultDevicePolicy.Enabled
	seen := make(map[string]struct{})
	for _, e := range eli {
		if !e.Status || e.Network != network.String() {
			continue
		}
		if !isDefaultPolicyActive {
			if !DoesUserHaveAccessToEgress(user, &e, acls) {
				continue
			}
		}
		if IsDomainBasedEgress(e) && HasEgressDomainAns(e) {
			for _, d := range ConfiguredDomainsForEgress(e) {
				d = normalizeEgressDomain(d)
				if d == "" {
					continue
				}
				if _, ok := seen[d]; ok {
					continue
				}
				seen[d] = struct{}{}
				domains = append(domains, d)
			}

		}
	}
	return
}

func GetEgressDomainNSForNode(ctx context.Context, node *models.Node) (returnNsLi []models.Nameserver) {
	acls := ListDevicePolicies(ctx, schema.NetworkID(node.Network))
	eli, _ := (&schema.Egress{Network: node.Network}).ListByNetwork(ctx)
	defaultDevicePolicy, _ := GetDefaultPolicy(ctx, schema.NetworkID(node.Network), models.DevicePolicy)
	isDefaultPolicyActive := defaultDevicePolicy.Enabled
	for _, e := range eli {
		if !e.Status || e.Network != node.Network {
			continue
		}
		if !isDefaultPolicyActive {
			if !DoesNodeHaveAccessToEgress(node, &e, acls) {
				continue
			}
		}
		if IsDomainBasedEgress(e) && HasEgressDomainAns(e) {
			var routingNodeIPs []string
			// Collect IPs from all routing nodes for this egress
			for nodeID := range e.Nodes {
				routingNode, err := GetNodeByID(nodeID)
				if err != nil {
					continue
				}
				if routingNode.ID == node.ID {
					continue
				}
				if routingNode.Address.IP != nil {
					routingNodeIPs = append(routingNodeIPs, routingNode.Address.IP.String())
				}
				if routingNode.Address6.IP != nil {
					routingNodeIPs = append(routingNodeIPs, routingNode.Address6.IP.String())
				}
			}
			for _, d := range ConfiguredDomainsForEgress(e) {
				d = normalizeEgressDomain(d)
				if d == "" {
					continue
				}
				returnNsLi = append(returnNsLi, models.Nameserver{
					IPs:            routingNodeIPs,
					MatchDomain:    d,
					IsSearchDomain: false,
				})
			}

		}
	}
	return
}

func GetNodeEgressInfo(targetNode *models.Node, eli []schema.Egress, acls []models.Acl) {

	req := models.EgressGatewayRequest{
		NodeID:     targetNode.ID.String(),
		NetID:      targetNode.Network,
		NatEnabled: "yes",
	}
	nodeTagIDs := snapshotNodeTagIDs(targetNode)
	// For internet egress, ::/0 is gated on the exit host's public IPv6 endpoint.
	includeIPv6 := exitHostHasEndpointIPv6(targetNode)
	for _, e := range eli {
		if !e.Status || e.Network != targetNode.Network {
			continue
		}
		if metric, ok := e.Nodes[targetNode.ID.String()]; ok {
			m64, err := metric.(json.Number).Int64()
			if err != nil {
				m64 = 256
			}
			appendEgressRangesToReq(&req, e, uint32(m64), includeIPv6)
		}
		for _, tagID := range nodeTagIDs {
			if metric, ok := e.Tags[tagID.String()]; ok {
				m64, err := metric.(json.Number).Int64()
				if err != nil {
					m64 = 256
				}
				appendEgressRangesToReq(&req, e, uint32(m64), includeIPv6)
				break
			}
		}
	}
	if targetNode.Mutex != nil {
		targetNode.Mutex.Lock()
	}
	if len(req.Ranges) > 0 {
		targetNode.EgressDetails.IsEgressGateway = true
		targetNode.EgressDetails.EgressGatewayRanges = req.Ranges
		targetNode.EgressDetails.EgressGatewayRequest = req
	} else {
		targetNode.EgressDetails = models.EgressDetails{}
	}
	if targetNode.Mutex != nil {
		targetNode.Mutex.Unlock()
	}
}

func RemoveNodeFromEgress(node models.Node) {
	ctx := db.WithContext(context.TODO())
	egs, err := (&schema.Egress{Network: node.Network}).ListByNetwork(ctx)
	if err != nil {
		slog.Error("RemoveNodeFromEgress: failed to list egresses", "error", err.Error())
		return
	}
	for i := range egs {
		egI := &egs[i]
		if _, ok := egI.Nodes[node.ID.String()]; ok {
			delete(egI.Nodes, node.ID.String())
			// Build a new map to ensure GORM persists the change; in-place modification
			// of the same map reference may not be detected by Updates(&struct).
			newNodes := make(datatypes.JSONMap)
			for k, v := range egI.Nodes {
				newNodes[k] = v
			}
			if err := db.FromContext(ctx).Table(egI.Table()).Where("id = ?", egI.ID).Updates(map[string]any{
				"nodes": newNodes,
			}).Error; err != nil {
				slog.Error("RemoveNodeFromEgress: failed to update egress", "id", egI.ID, "error", err.Error())
			}
		}
	}
}

func RemoveNodeFromEnrollmentKeys(node *models.Node) {
	_node := &schema.Node{
		ID: node.ID.String(),
	}
	_ = _node.ClearGatewayIDFromEnrollmentKeys(db.WithContext(context.TODO()))
}

func GetEgressRanges(ctx context.Context, netID schema.NetworkID) (map[string][]string, map[string]struct{}, error) {

	resultMap := make(map[string]struct{})
	nodeEgressMap := make(map[string][]string)
	networkNodes, err := GetNetworkNodes(ctx, netID.String())
	if err != nil {
		return nil, nil, err
	}
	for _, currentNode := range networkNodes {
		if currentNode.Network != netID.String() {
			continue
		}
		if currentNode.EgressDetails.IsEgressGateway { // add the egress gateway range(s) to the result
			if len(currentNode.EgressDetails.EgressGatewayRanges) > 0 {
				nodeEgressMap[currentNode.ID.String()] = currentNode.EgressDetails.EgressGatewayRanges
				for _, egressRangeI := range currentNode.EgressDetails.EgressGatewayRanges {
					resultMap[egressRangeI] = struct{}{}
				}
			}
		}
	}
	extclients, _ := GetNetworkExtClients(ctx, netID.String())
	for _, extclient := range extclients {
		if len(extclient.ExtraAllowedIPs) > 0 {
			nodeEgressMap[extclient.ClientID] = extclient.ExtraAllowedIPs
			for _, extraAllowedIP := range extclient.ExtraAllowedIPs {
				resultMap[extraAllowedIP] = struct{}{}
			}
		}
	}
	return nodeEgressMap, resultMap, nil
}

func ListAllByRoutingNodeWithDomain(egs []schema.Egress, nodeID string) (egWithDomain []models.EgressDomain) {
	node, err := GetNodeByID(nodeID)
	if err != nil {
		return
	}
	host := &schema.Host{
		ID: node.HostID,
	}
	err = host.Get(db.WithContext(context.TODO()))
	if err != nil {
		return
	}
	for _, egI := range egs {
		if !egI.Status || !IsDomainBasedEgress(egI) {
			continue
		}
		if _, ok := egI.Nodes[nodeID]; ok {
			for _, d := range ConfiguredDomainsForEgress(egI) {
				egWithDomain = append(egWithDomain, models.EgressDomain{
					ID:          egI.ID,
					Domain:      d,
					ResolvedIPs: DomainAnsForDomain(egI, d),
					Node:        node,
					Host:        *host,
				})
			}

		}
	}
	return
}

func normalizeEgressDomain(domain string) string {
	return strings.TrimSpace(strings.ToLower(domain))
}
