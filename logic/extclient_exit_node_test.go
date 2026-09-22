package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func TestApplyExtClientInternetEgressSelection_DashboardOmitLeavesEmpty(t *testing.T) {
	prev := findInternetEgressByRoutingNodeFn
	t.Cleanup(func() { findInternetEgressByRoutingNodeFn = prev })

	called := false
	findInternetEgressByRoutingNodeFn = func(ctx context.Context, network, nodeID string) (*schema.Egress, error) {
		called = true
		return &schema.Egress{ID: "eg-1"}, nil
	}

	client := &models.ExtClient{Network: "net-1", IngressGatewayID: "gw-1"}
	err := ApplyExtClientInternetEgressSelection(context.Background(), client, "gw-1", &models.CustomExtClient{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.SelectedInternetEgressID != "" {
		t.Fatalf("dashboard config must remain opt-in, got %q", client.SelectedInternetEgressID)
	}
	if called {
		t.Fatal("dashboard create must not auto-lookup internet egress")
	}
}

func TestApplyExtClientInternetEgressSelection_LegacyDeviceIDAutoSelects(t *testing.T) {
	prev := findInternetEgressByRoutingNodeFn
	t.Cleanup(func() { findInternetEgressByRoutingNodeFn = prev })

	findInternetEgressByRoutingNodeFn = func(ctx context.Context, network, nodeID string) (*schema.Egress, error) {
		if network != "net-1" || nodeID != "gw-1" {
			t.Fatalf("unexpected lookup args network=%s nodeID=%s", network, nodeID)
		}
		return &schema.Egress{ID: "eg-inet"}, nil
	}

	client := &models.ExtClient{Network: "net-1", IngressGatewayID: "gw-1"}
	err := ApplyExtClientInternetEgressSelection(context.Background(), client, "gw-1", &models.CustomExtClient{
		DeviceID: "device-abc",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.SelectedInternetEgressID != "eg-inet" {
		t.Fatalf("expected auto-selected egress, got %q", client.SelectedInternetEgressID)
	}
}

func TestApplyExtClientInternetEgressSelection_LegacyRACIDAutoSelects(t *testing.T) {
	prev := findInternetEgressByRoutingNodeFn
	t.Cleanup(func() { findInternetEgressByRoutingNodeFn = prev })

	findInternetEgressByRoutingNodeFn = func(ctx context.Context, network, nodeID string) (*schema.Egress, error) {
		return &schema.Egress{ID: "eg-inet"}, nil
	}

	client := &models.ExtClient{Network: "net-1", IngressGatewayID: "gw-1"}
	err := ApplyExtClientInternetEgressSelection(context.Background(), client, "gw-1", &models.CustomExtClient{
		RemoteAccessClientID: "rac-mac",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.SelectedInternetEgressID != "eg-inet" {
		t.Fatalf("expected auto-selected egress, got %q", client.SelectedInternetEgressID)
	}
}

func TestApplyExtClientInternetEgressSelection_LegacyNoExitGatewayLeavesEmpty(t *testing.T) {
	prev := findInternetEgressByRoutingNodeFn
	t.Cleanup(func() { findInternetEgressByRoutingNodeFn = prev })

	findInternetEgressByRoutingNodeFn = func(ctx context.Context, network, nodeID string) (*schema.Egress, error) {
		return nil, errors.New("internet egress not found for routing node")
	}

	client := &models.ExtClient{Network: "net-1", IngressGatewayID: "gw-1"}
	err := ApplyExtClientInternetEgressSelection(context.Background(), client, "gw-1", &models.CustomExtClient{
		DeviceID: "device-abc",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.SelectedInternetEgressID != "" {
		t.Fatalf("expected empty selection when gateway is not an exit node, got %q", client.SelectedInternetEgressID)
	}
}

func TestApplyExtClientInternetEgressSelection_PreservesExistingWhenOmitted(t *testing.T) {
	prev := findInternetEgressByRoutingNodeFn
	t.Cleanup(func() { findInternetEgressByRoutingNodeFn = prev })

	findInternetEgressByRoutingNodeFn = func(ctx context.Context, network, nodeID string) (*schema.Egress, error) {
		t.Fatal("must not re-lookup when selection already set")
		return nil, nil
	}

	client := &models.ExtClient{
		Network:                  "net-1",
		IngressGatewayID:         "gw-1",
		SelectedInternetEgressID: "eg-existing",
	}
	err := ApplyExtClientInternetEgressSelection(context.Background(), client, "gw-1", &models.CustomExtClient{
		DeviceID: "device-abc",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.SelectedInternetEgressID != "eg-existing" {
		t.Fatalf("expected existing selection preserved, got %q", client.SelectedInternetEgressID)
	}
}

func TestApplyExtClientInternetEgressSelection_ExplicitOptOutClears(t *testing.T) {
	off := false
	client := &models.ExtClient{
		Network:                  "net-1",
		IngressGatewayID:         "gw-1",
		SelectedInternetEgressID: "eg-existing",
	}
	err := ApplyExtClientInternetEgressSelection(context.Background(), client, "gw-1", &models.CustomExtClient{
		DeviceID:          "device-abc",
		UseInternetEgress: &off,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.SelectedInternetEgressID != "" {
		t.Fatalf("expected selection cleared, got %q", client.SelectedInternetEgressID)
	}
}
