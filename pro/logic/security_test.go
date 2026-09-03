package logic

import (
	"net/http"
	"testing"

	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func TestIsDeviceAPIRequest(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/api/v1/device/networks", true},
		{"/api/v1/device/networks/net1/join", true},
		{"/api/v1/device/register", true},
		{"/api/v1/devices/networks", false},
		{"/api/v1/host/device/foo", false},
		{"/api/networks", false},
		{"/api/v1/device", false},
	}
	for _, tt := range tests {
		req, err := http.NewRequest(http.MethodGet, tt.path, nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		if got := isDeviceAPIRequest(req); got != tt.want {
			t.Fatalf("path %q: got %v want %v", tt.path, got, tt.want)
		}
	}
}

func TestRoleGrantsDeviceWrite(t *testing.T) {
	if !roleGrantsDeviceWrite(&schema.UserRole{TenantGlobalAccess: true}) {
		t.Fatal("full access should grant write")
	}

	readOnly := &schema.UserRole{
		NetworkLevelAccess: datatypes.NewJSONType(schema.ResourceAccess{
			schema.HostRsrc: {
				schema.AllHostRsrcID: {Read: true},
			},
			schema.EgressGwRsrc: {
				schema.AllEgressGwRsrcID: {Read: true},
			},
		}),
	}
	if roleGrantsDeviceWrite(readOnly) {
		t.Fatal("read-only role should not grant device write")
	}

	networkUser := &schema.UserRole{
		NetworkLevelAccess: datatypes.NewJSONType(schema.ResourceAccess{
			schema.HostRsrc: {
				schema.AllHostRsrcID: {Read: true},
			},
			schema.RemoteAccessGwRsrc: {
				schema.AllRemoteAccessGwRsrcID: {Read: true, VPNaccess: true},
			},
		}),
	}
	if !roleGrantsDeviceWrite(networkUser) {
		t.Fatal("network user with VPNaccess should grant device write")
	}

	hostWriter := &schema.UserRole{
		NetworkLevelAccess: datatypes.NewJSONType(schema.ResourceAccess{
			schema.HostRsrc: {
				schema.AllHostRsrcID: {Read: true, Update: true},
			},
		}),
	}
	if !roleGrantsDeviceWrite(hostWriter) {
		t.Fatal("host update should grant device write")
	}
}
