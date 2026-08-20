package edr

import (
	"net"
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
		Name:          "editable-hostname",
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

func TestHostEligibleForEDR_Wazuh(t *testing.T) {
	if hostEligibleForEDR(ProviderWazuh, schema.Host{}) {
		t.Fatal("wazuh requires registered host id")
	}
	if !hostEligibleForEDR(ProviderWazuh, schema.Host{ID: uuid.New()}) {
		t.Fatal("wazuh eligible with host id")
	}
}

func TestMatchHostToEndpoint_WazuhHostIDByName(t *testing.T) {
	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	host := schema.Host{ID: id, Name: "web-01"}
	ep := ManagedEndpoint{Hostname: id.String(), EndpointIP: "10.0.0.99"}
	matchedBy, ok := MatchHostToEndpoint(ProviderWazuh, host, ep)
	if !ok || matchedBy != schema.EDRMatchHostID {
		t.Fatalf("expected host_id match, got %q ok=%v", matchedBy, ok)
	}
}

func TestMatchHostToEndpoint_WazuhHostIDByLabel(t *testing.T) {
	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	host := schema.Host{ID: id, Name: "web-01"}
	ep := ManagedEndpoint{
		Hostname:       "web-01",
		EndpointIP:     "10.0.0.1",
		NetmakerHostID: id.String(),
	}
	matchedBy, ok := MatchHostToEndpoint(ProviderWazuh, host, ep)
	if !ok || matchedBy != schema.EDRMatchHostID {
		t.Fatalf("expected host_id label match, got %q ok=%v", matchedBy, ok)
	}
}

func TestMatchHostToEndpoint_WazuhHostnameOnlyRejected(t *testing.T) {
	host := schema.Host{ID: uuid.New(), Name: "web-01", EndpointIP: net.ParseIP("10.0.0.5")}
	ep := ManagedEndpoint{Hostname: "web-01", EndpointIP: "10.0.0.9"}
	if _, ok := MatchHostToEndpoint(ProviderWazuh, host, ep); ok {
		t.Fatal("expected no match when hostname matches but endpoint ip differs")
	}
}

func TestMatchHostToEndpoint_WazuhHostnameAndEndpoint(t *testing.T) {
	host := schema.Host{
		ID:         uuid.New(),
		Name:       "web-01",
		EndpointIP: net.ParseIP("10.0.0.5"),
	}
	ep := ManagedEndpoint{Hostname: "web-01", EndpointIP: "10.0.0.5"}
	matchedBy, ok := MatchHostToEndpoint(ProviderWazuh, host, ep)
	if !ok || matchedBy != schema.EDRMatchHostname {
		t.Fatalf("expected hostname match, got %q ok=%v", matchedBy, ok)
	}
}

func TestMatchHostToEndpoint_WazuhHostnameRejectedForBadAgentIP(t *testing.T) {
	host := schema.Host{
		ID:         uuid.New(),
		Name:       "web-01",
		EndpointIP: net.ParseIP("10.0.0.5"),
	}
	for _, agentIP := range []string{"", "any", "127.0.0.1"} {
		ep := ManagedEndpoint{Hostname: "web-01", EndpointIP: agentIP}
		if _, ok := MatchHostToEndpoint(ProviderWazuh, host, ep); ok {
			t.Fatalf("expected no match for agent ip %q", agentIP)
		}
	}
}

func TestHostEndpointIPMatches(t *testing.T) {
	host := schema.Host{EndpointIP: net.ParseIP("203.0.113.10"), EndpointIPv6: net.ParseIP("2001:db8::1")}
	if !HostEndpointIPMatches(host, "203.0.113.10") {
		t.Fatal("expected ipv4 match")
	}
	if !HostEndpointIPMatches(host, "2001:db8::1") {
		t.Fatal("expected ipv6 match")
	}
	if HostEndpointIPMatches(host, "10.0.0.1") {
		t.Fatal("expected no match for different ip")
	}
	if HostEndpointIPMatches(host, "any") {
		t.Fatal("expected any to be rejected")
	}
}
