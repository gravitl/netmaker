// Package mdm defines the pluggable MDM provider interface and registry used by
// the Netmaker MDM posture-check feature. Concrete providers (Intune, Jamf,
// future Iru/JumpCloud/etc.) live in sibling packages and self-register via
// init().
package mdm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ManagedDevice is the normalised, provider-agnostic view of a device that an
// MDM Provider returns. Fields that a given provider can't fill are left as
// their zero value.
type ManagedDevice struct {
	// ProviderDeviceID is the primary key in the upstream MDM.
	ProviderDeviceID string
	// AzureADDeviceID is filled by Intune; non-Entra MDMs leave it blank.
	AzureADDeviceID string

	SerialNumber      string
	HardwareUUID      string
	DeviceName        string
	UserPrincipalName string // user email

	Enrolled   bool
	Compliant  bool
	LastSeenAt time.Time
}

// Capabilities advertises optional provider features so callers (UI / API)
// know what to surface.
type Capabilities struct {
	// ReportsCompliant is true if the provider populates ManagedDevice.Compliant
	// with a meaningful value derived from upstream compliance state. When
	// false, callers should treat Compliant as "unknown" rather than "false".
	ReportsCompliant bool
}

// Provider is the minimal contract every MDM integration must satisfy.
type Provider interface {
	// Name returns the stable identifier of this provider (matches integrations_v1.id).
	Name() string
	// Capabilities advertises optional provider features.
	Capabilities() Capabilities
	// Verify confirms credentials and connectivity against the upstream MDM.
	Verify(ctx context.Context) error
	// ListManagedDevices returns every device known to the upstream MDM.
	ListManagedDevices(ctx context.Context) ([]ManagedDevice, error)
}

// ProviderType describes a provider implementation available at compile time.
type ProviderType struct {
	Name             string `json:"name"`
	Display          string `json:"display"`
	ReportsCompliant bool   `json:"reports_compliant"`
}

// Factory builds a Provider instance from integration config JSON.
type Factory func(config json.RawMessage) (Provider, error)

type providerEntry struct {
	display string
	factory Factory
}

var providers = map[string]providerEntry{}

// Register binds a provider implementation to its stable name.
func Register(name, display string, f Factory) {
	providers[name] = providerEntry{display: display, factory: f}
}

// ListProviderTypes returns the registered providers with capability flags.
func ListProviderTypes() []ProviderType {
	out := make([]ProviderType, 0, len(providers))
	for name, entry := range providers {
		pt := ProviderType{Name: name, Display: entry.display}
		if c, ok := capabilityHints[name]; ok {
			pt.ReportsCompliant = c.ReportsCompliant
		}
		out = append(out, pt)
	}
	return out
}

var capabilityHints = map[string]Capabilities{}

// RegisterCapabilities records the static capability profile of a provider.
func RegisterCapabilities(name string, c Capabilities) {
	capabilityHints[name] = c
}

// CapabilitiesFor returns the registered capability profile for a provider id.
func CapabilitiesFor(name string) Capabilities {
	if c, ok := capabilityHints[name]; ok {
		return c
	}
	return Capabilities{}
}

// Build constructs a provider by explicit name from config JSON.
func Build(name string, config json.RawMessage) (Provider, error) {
	entry, ok := providers[name]
	if !ok {
		return nil, fmt.Errorf("unknown mdm provider %q", name)
	}
	return entry.factory(config)
}
