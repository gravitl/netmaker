package logic

import (
	"context"
	"errors"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

// ExtClientUsesInternetEgress reports whether the config file should route all traffic via its gateway exit node.
func ExtClientUsesInternetEgress(client models.ExtClient, gwnode models.Node) bool {
	if client.SelectedInternetEgressID != "" {
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
	// Legacy internet gateway flag: all config files on the gateway use full tunnel.
	return gwnode.IsInternetGateway
}

// ApplyExtClientInternetEgressSelection resolves use_internet_egress on create or validates selected_internet_egress_id.
func ApplyExtClientInternetEgressSelection(ctx context.Context, client *models.ExtClient, gatewayNodeID string, update *models.CustomExtClient) error {
	if client == nil || update == nil {
		return errors.New("client is required")
	}
	if gatewayNodeID == "" {
		gatewayNodeID = client.IngressGatewayID
	}
	if update.UseInternetEgress != nil {
		if *update.UseInternetEgress {
			e, err := FindInternetEgressByRoutingNode(ctx, client.Network, gatewayNodeID)
			if err != nil {
				return errors.New("gateway is not an internet exit node")
			}
			client.SelectedInternetEgressID = e.ID
			return nil
		}
		client.SelectedInternetEgressID = ""
		return nil
	}
	if update.SelectedInternetEgressID == "" {
		return nil
	}
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
