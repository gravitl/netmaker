package edr

import (
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/schema"
)

func TestMatchHostToEndpoint_Serial(t *testing.T) {
	host := schema.Host{ID: uuid.New(), SerialNumber: "SN-1", Name: "host-a"}
	ep := ManagedEndpoint{SerialNumber: "sn-1", Hostname: "other"}
	if _, ok := MatchHostToEndpoint(ProviderCrowdStrike, host, ep); !ok {
		t.Fatal("expected serial match")
	}
}

func TestMatchHostToEndpoint_NoHostnameFallback(t *testing.T) {
	host := schema.Host{ID: uuid.New(), Name: "Abhisheks-MacBook-Pro.local"}
	ep := ManagedEndpoint{Hostname: "Abhisheks-MacBook-Pro.local", SerialNumber: "FVFGT7SGQ05P"}
	if _, ok := MatchHostToEndpoint(ProviderCrowdStrike, host, ep); ok {
		t.Fatal("expected no hostname-only match")
	}
	if _, ok := MatchHostToEndpoint(ProviderSentinelOne, host, ep); ok {
		t.Fatal("expected no hostname-only match for sentinelone")
	}
	host.SerialNumber = "FVFGT7SGQ05P"
	if _, ok := MatchHostToEndpoint(ProviderCrowdStrike, host, ep); !ok {
		t.Fatal("expected serial match")
	}
}

func TestMatchHostToEndpoint_DefenderEntra(t *testing.T) {
	host := schema.Host{
		ID:            uuid.New(),
		EntraDeviceID: "32f5f9ec-cd23-41e0-94e8-6b372232ff40",
		Name:            "editable-hostname",
	}
	ep := ManagedEndpoint{
		EntraDeviceID: "32f5f9ec-cd23-41e0-94e8-6b372232ff40",
		Hostname:      "different-dns-name",
	}
	matchedBy, ok := MatchHostToEndpoint(ProviderDefender, host, ep)
	if !ok || matchedBy != schema.EDRMatchEntraDeviceID {
		t.Fatalf("expected entra match, got %q ok=%v", matchedBy, ok)
	}
}

func TestMatchHostToEndpoint_DefenderNoHostnameFallback(t *testing.T) {
	host := schema.Host{ID: uuid.New(), Name: "win-pc.corp.local"}
	ep := ManagedEndpoint{Hostname: "win-pc.corp.local"}
	if _, ok := MatchHostToEndpoint(ProviderDefender, host, ep); ok {
		t.Fatal("defender must not match by hostname")
	}
}

func TestMatchHostToEndpoint_DefenderSerial(t *testing.T) {
	host := schema.Host{
		ID:           uuid.New(),
		SerialNumber: "SN-42",
	}
	ep := ManagedEndpoint{
		SerialNumber: "sn-42",
		Hostname:     "other-host",
	}
	matchedBy, ok := MatchHostToEndpoint(ProviderDefender, host, ep)
	if !ok || matchedBy != schema.EDRMatchSerialNumber {
		t.Fatalf("expected defender serial match, got %q ok=%v", matchedBy, ok)
	}
}

func TestHostEligibleForEDR(t *testing.T) {
	if hostEligibleForEDR(ProviderDefender, schema.Host{Name: "host-only"}) {
		t.Fatal("defender requires entra_device_id or serial_number")
	}
	if !hostEligibleForEDR(ProviderDefender, schema.Host{EntraDeviceID: "guid"}) {
		t.Fatal("defender eligible with entra_device_id")
	}
	if !hostEligibleForEDR(ProviderDefender, schema.Host{SerialNumber: "SN1"}) {
		t.Fatal("defender eligible with serial_number")
	}
	if hostEligibleForEDR(ProviderCrowdStrike, schema.Host{Name: "host-only"}) {
		t.Fatal("crowdstrike requires serial_number")
	}
	if !hostEligibleForEDR(ProviderCrowdStrike, schema.Host{SerialNumber: "SN1"}) {
		t.Fatal("crowdstrike eligible with serial_number")
	}
}
