package logic

import (
	"context"
	"errors"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

// findInternetEgressByRoutingNodeFn looks up an internet egress by routing node.
// Overridable in unit tests.
var findInternetEgressByRoutingNodeFn = FindInternetEgressByRoutingNode

// ExtClientUsesInternetEgress reports whether the config file should route all traffic via its gateway exit node.
//
// Using the gateway as an exit node is opt-in per client via SelectedInternetEgressID
// (the "Use gateway as exit node" toggle). It must NOT be forced on just because the
// gateway happens to be an internet gateway/exit node; clients that did not opt in
// should not get a full tunnel. Legacy full-tunnel clients are migrated to an explicit
// SelectedInternetEgressID, so no gateway-level fallback is needed.
func ExtClientUsesInternetEgress(client models.ExtClient, _ models.Node) bool {
	if client.SelectedInternetEgressID == "" {
		return false
	}
	e := &schema.Egress{ID: client.SelectedInternetEgressID}
	if err := e.Get(db.WithContext(context.TODO())); err != nil {
		return false
	}
	if !e.Status || e.Network != client.Network || !IsEgressInternetGateway(*e) {
		return false
	}
	_, ok := e.Nodes[client.IngressGatewayID]
	return ok
}

// ApplyExtClientInternetEgressSelection resolves use_internet_egress on create/update
// or validates selected_internet_egress_id. Opt-in is per config file.
//
// Legacy desktop/RAC apps omit use_internet_egress. When those clients connect through
// a gateway that is also an exit node, the gateway's internet egress is auto-selected
// so AllowedIPs include 0.0.0.0/0 (previous IsInternetGateway behavior). Dashboard
// config files (no device_id / remote_access_client_id) remain opt-in only.
func ApplyExtClientInternetEgressSelection(ctx context.Context, client *models.ExtClient, gatewayNodeID string, update *models.CustomExtClient) error {
	if client == nil || update == nil {
		return errors.New("client is required")
	}
	if gatewayNodeID == "" {
		gatewayNodeID = client.IngressGatewayID
	}
	if update.UseInternetEgress != nil {
		if !*update.UseInternetEgress {
			client.SelectedInternetEgressID = ""
			return nil
		}
		// Prefer an explicit egress id from the client when provided.
		if update.SelectedInternetEgressID != "" {
			e := &schema.Egress{ID: update.SelectedInternetEgressID}
			if err := e.Get(ctx); err != nil {
				return errors.New("exit node not found")
			}
			if !e.Status || e.Network != client.Network || !IsEgressInternetGateway(*e) {
				return errors.New("egress is not an active internet exit node in this network")
			}
			if _, ok := e.Nodes[gatewayNodeID]; !ok {
				return errors.New("selected exit node is not provided by this gateway")
			}
			client.SelectedInternetEgressID = update.SelectedInternetEgressID
			return nil
		}
		e, err := findInternetEgressByRoutingNodeFn(ctx, client.Network, gatewayNodeID)
		if err != nil {
			return errors.New("gateway is not an internet exit node")
		}
		client.SelectedInternetEgressID = e.ID
		return nil
	}
	if update.SelectedInternetEgressID != "" {
		e := &schema.Egress{ID: update.SelectedInternetEgressID}
		if err := e.Get(ctx); err != nil {
			return errors.New("exit node not found")
		}
		if !e.Status || e.Network != client.Network || !IsEgressInternetGateway(*e) {
			return errors.New("egress is not an active internet exit node in this network")
		}
		if _, ok := e.Nodes[gatewayNodeID]; !ok {
			return errors.New("selected exit node is not provided by this gateway")
		}
		client.SelectedInternetEgressID = update.SelectedInternetEgressID
		return nil
	}
	// No explicit choice: legacy desktop/RAC create through an exit-node gateway
	// auto-selects that egress. Dashboard configs and updates that already have a
	// selection (or none) are left unchanged.
	if client.SelectedInternetEgressID == "" &&
		(update.DeviceID != "" || update.RemoteAccessClientID != "") {
		if e, err := findInternetEgressByRoutingNodeFn(ctx, client.Network, gatewayNodeID); err == nil {
			client.SelectedInternetEgressID = e.ID
		}
	}
	return nil
}
