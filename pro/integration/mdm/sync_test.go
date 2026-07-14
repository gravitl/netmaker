package mdm

import (
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/schema"
)

func TestMatchHostToMDMDeviceBySerial(t *testing.T) {
	host := schema.Host{
		ID:           uuid.New(),
		SerialNumber: " ABC123 ",
	}
	device := ManagedDevice{SerialNumber: "abc123"}
	if !MatchHostToMDMDeviceBySerial(host, device) {
		t.Fatal("expected serial match")
	}
}

func TestMatchHostToMDMDeviceBySerial_NoMatch(t *testing.T) {
	host := schema.Host{
		ID:           uuid.New(),
		SerialNumber: "ABC123",
	}
	device := ManagedDevice{SerialNumber: "XYZ999"}
	if MatchHostToMDMDeviceBySerial(host, device) {
		t.Fatal("expected no match")
	}
}

func TestMatchHostToMDMDeviceBySerial_EmptyHostSerial(t *testing.T) {
	host := schema.Host{ID: uuid.New()}
	device := ManagedDevice{SerialNumber: "ABC123"}
	if MatchHostToMDMDeviceBySerial(host, device) {
		t.Fatal("expected no match when host serial is empty")
	}
}
