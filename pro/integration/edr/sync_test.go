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

func TestMatchHostToEndpoint_DefenderEntra(t *testing.T) {
	host := schema.Host{
		ID:            uuid.New(),
		EntraDeviceID: "32f5f9ec-cd23-41e0-94e8-6b372232ff40",
	}
	ep := ManagedEndpoint{EntraDeviceID: "32f5f9ec-cd23-41e0-94e8-6b372232ff40"}
	matchedBy, ok := MatchHostToEndpoint(ProviderDefender, host, ep)
	if !ok || matchedBy != schema.EDRMatchEntraDeviceID {
		t.Fatalf("expected entra match, got %q ok=%v", matchedBy, ok)
	}
}

func TestHostnameMatch_StripsDomain(t *testing.T) {
	if !hostnameMatch("WIN-PC.corp.local", "win-pc") {
		t.Fatal("expected hostname match without domain")
	}
}
