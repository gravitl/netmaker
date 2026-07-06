// Package edr defines the pluggable EDR provider interface and registry used by
// the Netmaker EDR posture-check feature.
package edr

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ManagedEndpoint is the provider-agnostic view of an endpoint returned by EDR
// integrations. Normalized posture fields are populated by each provider.
type ManagedEndpoint struct {
	ProviderDeviceID string
	SerialNumber     string
	Hostname         string
	EntraDeviceID    string

	AgentInstalled bool
	AgentHealthy   bool
	RiskLevel      RiskLevel
	ThreatCount    int
	ActiveThreats  bool
	Isolated       bool
	Contained      bool
	LastSeen       time.Time

	RawVendorData json.RawMessage
}

// Capabilities advertises optional provider features.
type Capabilities struct {
	ReportsRisk bool
}

// Provider is the minimal contract every EDR integration must satisfy.
type Provider interface {
	Name() string
	Capabilities() Capabilities
	Verify(ctx context.Context) error
	ListManagedEndpoints(ctx context.Context) ([]ManagedEndpoint, error)
}

// ProviderType describes a registered provider for API listing.
type ProviderType struct {
	Name        string `json:"name"`
	Display     string `json:"display"`
	ReportsRisk bool   `json:"reports_risk"`
}

type Factory func(config json.RawMessage) (Provider, error)

type providerEntry struct {
	display string
	factory Factory
}

var providers = map[string]providerEntry{}

func Register(name, display string, f Factory) {
	providers[name] = providerEntry{display: display, factory: f}
}

var capabilityHints = map[string]Capabilities{}

func RegisterCapabilities(name string, c Capabilities) {
	capabilityHints[name] = c
}

func CapabilitiesFor(name string) Capabilities {
	if c, ok := capabilityHints[name]; ok {
		return c
	}
	return Capabilities{}
}

func ListProviderTypes() []ProviderType {
	out := make([]ProviderType, 0, len(providers))
	for name, entry := range providers {
		pt := ProviderType{Name: name, Display: entry.display}
		if c, ok := capabilityHints[name]; ok {
			pt.ReportsRisk = c.ReportsRisk
		}
		out = append(out, pt)
	}
	return out
}

func Build(name string, config json.RawMessage) (Provider, error) {
	entry, ok := providers[name]
	if !ok {
		return nil, fmt.Errorf("unknown edr provider %q", name)
	}
	return entry.factory(config)
}
