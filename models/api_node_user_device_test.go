package models

import (
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/schema"
)

func TestConvertToAPINode_marksUserOwnedHostAsUserNode(t *testing.T) {
	hostID := uuid.New()
	nodeID := uuid.New()
	nm := Node{
		CommonNode: CommonNode{
			ID:      nodeID,
			HostID:  hostID,
			Network: "netmaker",
			Address: net.IPNet{IP: net.ParseIP("10.0.0.5"), Mask: net.CIDRMask(32, 32)},
		},
		OwnerID: "alice",
		StaticNode: ExtClient{
			OwnerID:    "alice",
			DeviceID:   hostID.String(),
			DeviceName: "alice-laptop",
			OS:         "darwin",
			ClientID:   "alice-laptop",
			Network:    "netmaker",
			Address:    "10.0.0.5",
			Enabled:    true,
			Status:     schema.OnlineSt,
		},
		Status: schema.OnlineSt,
	}

	api := nm.ConvertToAPINode()
	if !api.IsUserNode {
		t.Fatal("expected is_user_node true for host with OwnerID")
	}
	if api.IsStatic {
		t.Fatal("host-backed user devices must not be is_static")
	}
	if api.StaticNode.OwnerID != "alice" {
		t.Fatalf("expected ownerid alice, got %q", api.StaticNode.OwnerID)
	}
	if api.StaticNode.DeviceID != hostID.String() {
		t.Fatalf("expected device_id %s, got %q", hostID, api.StaticNode.DeviceID)
	}
}

func TestConvertToAPINode_legacyExtClientUnchanged(t *testing.T) {
	nm := Node{
		IsStatic:   true,
		IsUserNode: true,
		StaticNode: ExtClient{
			OwnerID:              "bob",
			RemoteAccessClientID: "mac-1",
			ClientID:             "bob-client",
			Network:              "netmaker",
		},
	}
	api := nm.ConvertToAPINode()
	if !api.IsUserNode || !api.IsStatic {
		t.Fatalf("expected static user node, got is_user_node=%v is_static=%v", api.IsUserNode, api.IsStatic)
	}
	if api.StaticNode.RemoteAccessClientID != "mac-1" {
		t.Fatalf("unexpected rac id %q", api.StaticNode.RemoteAccessClientID)
	}
}

func TestConvertToStatusNode_marksUserOwnedHost(t *testing.T) {
	nm := Node{
		CommonNode: CommonNode{ID: uuid.New()},
		OwnerID:    "alice",
		Status:     schema.OnlineSt,
	}
	st := nm.ConvertToStatusNode()
	if !st.IsUserNode {
		t.Fatal("expected is_user_node on status DTO")
	}
	if st.IsStatic {
		t.Fatal("status DTO must not mark host-backed device as static")
	}
}

func TestConvertToAPINode_unownedHostNotUserNode(t *testing.T) {
	nm := Node{
		CommonNode: CommonNode{ID: uuid.New(), HostID: uuid.New()},
	}
	api := nm.ConvertToAPINode()
	if api.IsUserNode {
		t.Fatal("unowned host must not be is_user_node")
	}
}
