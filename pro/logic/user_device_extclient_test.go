package logic

import (
	"context"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func TestUserAllowedToAnyExtClientOnIngress(t *testing.T) {
	ingressID := uuid.New()
	ingress := models.Node{
		CommonNode: models.CommonNode{
			ID:      ingressID,
			Network: "netmaker",
		},
	}
	ingress.IsIngressGateway = true
	tagged := models.ExtClient{
		ClientID:         "wg-tagged",
		Network:          "netmaker",
		Address:          "100.64.0.50",
		Enabled:          true,
		IngressGatewayID: ingressID.String(),
		Tags:             map[models.TagID]struct{}{"site-a": {}},
	}
	rac := models.ExtClient{
		ClientID:             "rac-user",
		Network:              "netmaker",
		Address:              "100.64.0.51",
		Enabled:              true,
		IngressGatewayID:     ingressID.String(),
		RemoteAccessClientID: "mac",
		OwnerID:              "bob",
	}
	otherIngress := models.ExtClient{
		ClientID:         "other-gw",
		Network:          "netmaker",
		Address:          "100.64.0.52",
		Enabled:          true,
		IngressGatewayID: uuid.New().String(),
	}

	origList := listExtClientsForUserACL
	origAllow := logic.IsUserAllowedToCommunicate
	t.Cleanup(func() {
		listExtClientsForUserACL = origList
		logic.IsUserAllowedToCommunicate = origAllow
	})

	listExtClientsForUserACL = func(context.Context, string) ([]models.ExtClient, error) {
		return []models.ExtClient{tagged, rac, otherIngress}, nil
	}
	logic.IsUserAllowedToCommunicate = func(_ context.Context, userName string, peer models.Node) (bool, []models.Acl) {
		if userName != "alice" {
			return false, nil
		}
		if peer.IsStatic && peer.StaticNode.ClientID == "wg-tagged" {
			return true, []models.Acl{{ID: "user-to-tag", Enabled: true}}
		}
		return false, nil
	}

	ok, policies := userAllowedToAnyExtClientOnIngress(context.Background(), "alice", ingress)
	if !ok {
		t.Fatal("expected ingress allow when an attached tagged extclient is permitted")
	}
	if len(policies) != 1 || policies[0].ID != "user-to-tag" {
		t.Fatalf("unexpected policies: %+v", policies)
	}

	listExtClientsForUserACL = func(context.Context, string) ([]models.ExtClient, error) {
		return []models.ExtClient{rac, otherIngress}, nil
	}
	ok, _ = userAllowedToAnyExtClientOnIngress(context.Background(), "alice", ingress)
	if ok {
		t.Fatal("RAC extclients must not grant ingress peering for user devices")
	}

	nonIngress := ingress
	nonIngress.IsIngressGateway = false
	ok, _ = userAllowedToAnyExtClientOnIngress(context.Background(), "alice", nonIngress)
	if ok {
		t.Fatal("non-ingress peers must not use the extclient bridge")
	}
}

func TestGetFwRulesForUserDevicesOnGw_IncludesNonRelayedDeviceToExtClient(t *testing.T) {
	gwID := uuid.New()
	gw := models.Node{
		CommonNode: models.CommonNode{
			ID:      gwID,
			Network: "netmaker",
		},
	}
	gw.IsIngressGateway = true
	device := userDeviceNode("alice", "100.64.0.9")
	device.Connected = true
	device.RelayedBy = ""

	ext := models.ConvertToStaticNode(models.ExtClient{
		ClientID:         "wg-tagged",
		Network:          "netmaker",
		Address:          "100.64.0.50",
		Enabled:          true,
		IngressGatewayID: gwID.String(),
		Tags:             map[models.TagID]struct{}{"site-a": {}},
		ExtraAllowedIPs:  []string{"10.20.0.0/24"},
	})

	origDevices := listUserDevicesForACL
	origEgress := listEgressForUserDeviceACL
	origDefault := getDefaultUserPolicyForUserDeviceACL
	origAllow := logic.IsUserAllowedToCommunicate
	t.Cleanup(func() {
		listUserDevicesForACL = origDevices
		listEgressForUserDeviceACL = origEgress
		getDefaultUserPolicyForUserDeviceACL = origDefault
		logic.IsUserAllowedToCommunicate = origAllow
	})

	listUserDevicesForACL = func(context.Context, string) []models.Node {
		return []models.Node{device}
	}
	listEgressForUserDeviceACL = func(context.Context, string) ([]schema.Egress, error) {
		return nil, nil
	}
	getDefaultUserPolicyForUserDeviceACL = func(context.Context, schema.NetworkID) (models.Acl, error) {
		return models.Acl{Enabled: false}, nil
	}
	logic.IsUserAllowedToCommunicate = func(_ context.Context, userName string, peer models.Node) (bool, []models.Acl) {
		if userName != "alice" {
			return false, nil
		}
		if peer.IsStatic && peer.StaticNode.ClientID == "wg-tagged" {
			return true, []models.Acl{{
				ID:      "user-to-ext",
				Enabled: true,
				Proto:   models.ALL,
			}}
		}
		return false, nil
	}

	rules := GetFwRulesForUserDevicesOnGw(context.Background(), gw, []models.Node{device, ext})
	wantSrc := net.IPNet{IP: net.ParseIP("100.64.0.9"), Mask: net.CIDRMask(32, 32)}
	wantOverlay := net.IPNet{IP: net.ParseIP("100.64.0.50"), Mask: net.CIDRMask(32, 32)}
	wantExtra := "10.20.0.0/24"
	foundOverlay, foundExtra := false, false
	for _, r := range rules {
		if r.SrcIP.String() != wantSrc.String() || !r.Allow {
			continue
		}
		if r.DstIP.String() == wantOverlay.String() {
			foundOverlay = true
		}
		if r.DstIP.String() == wantExtra {
			foundExtra = true
		}
	}
	if !foundOverlay {
		t.Fatalf("expected fw rule %s -> %s for non-relayed user device, got %+v", wantSrc, wantOverlay, rules)
	}
	if !foundExtra {
		t.Fatalf("expected fw rule %s -> %s for extclient ExtraAllowedIPs, got %+v", wantSrc, wantExtra, rules)
	}
}
