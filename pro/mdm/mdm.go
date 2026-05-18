// Package mdm defines the pluggable MDM provider interface and registry used by
// the Netmaker MDM posture-check feature. Concrete providers (Intune, Jamf,
// future Kandji/JumpCloud/etc.) live in sibling packages and self-register via
// init().
package mdm

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitl/netmaker/models"
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
	// Name returns the stable identifier of this provider (matches the value
	// stored in ServerSettings.MDMProvider). Examples: "intune", "jamf".
	Name() string
	// Capabilities advertises optional provider features.
	Capabilities() Capabilities
	// Verify confirms credentials and connectivity against the upstream MDM.
	Verify(ctx context.Context) error
	// ListManagedDevices returns every device known to the upstream MDM.
	ListManagedDevices(ctx context.Context) ([]ManagedDevice, error)
}

// ProviderType describes a provider implementation available at compile time.
// Used by GET /api/v1/mdm/provider_types to populate the integrations UI.
type ProviderType struct {
	Name             string `json:"name"`
	Display          string `json:"display"`
	ReportsCompliant bool   `json:"reports_compliant"`
}

// Factory builds a Provider instance from the current ServerSettings. Each
// provider reads only its own field group (MDMIntune*, MDMJamf*, etc.).
type Factory func(s models.ServerSettings) (Provider, error)

// providerEntry holds the metadata Register accepts in one place.
type providerEntry struct {
	display string
	factory Factory
}

var providers = map[string]providerEntry{}

// Register binds a provider implementation to its stable name. Each provider
// package calls this from init() so the binary auto-discovers what's compiled
// in.
func Register(name, display string, f Factory) {
	providers[name] = providerEntry{display: display, factory: f}
}

// ListProviderTypes returns the registered providers, with their capability
// flags resolved by instantiating each one against a zero-valued
// ServerSettings (capability hints must not depend on stored credentials).
func ListProviderTypes() []ProviderType {
	out := make([]ProviderType, 0, len(providers))
	for name, entry := range providers {
		pt := ProviderType{Name: name, Display: entry.display}
		// Capabilities are static per provider; ask any concrete instance.
		if p, err := entry.factory(models.ServerSettings{}); err == nil && p != nil {
			pt.ReportsCompliant = p.Capabilities().ReportsCompliant
		} else if c, ok := capabilityHints[name]; ok {
			pt.ReportsCompliant = c.ReportsCompliant
		}
		out = append(out, pt)
	}
	return out
}

// capabilityHints lets providers advertise their capabilities without
// requiring valid credentials. Concrete provider packages populate this in
// their init() alongside Register.
var capabilityHints = map[string]Capabilities{}

// RegisterCapabilities records the static capability profile of a provider so
// ListProviderTypes can answer even when credentials are missing.
func RegisterCapabilities(name string, c Capabilities) {
	capabilityHints[name] = c
}

// BuildActive returns the provider selected by ServerSettings.MDMProvider, or
// (nil, nil) if no MDM is configured. Errors are reserved for "configured but
// invalid" cases (e.g. credentials missing for the named provider).
func BuildActive(s models.ServerSettings) (Provider, error) {
	if s.MDMProvider == "" {
		return nil, nil
	}
	entry, ok := providers[s.MDMProvider]
	if !ok {
		return nil, fmt.Errorf("unknown mdm provider %q", s.MDMProvider)
	}
	return entry.factory(s)
}

// Build constructs a provider by explicit name, useful for ad-hoc verify
// calls that supply a draft ServerSettings.
func Build(name string, s models.ServerSettings) (Provider, error) {
	entry, ok := providers[name]
	if !ok {
		return nil, fmt.Errorf("unknown mdm provider %q", name)
	}
	return entry.factory(s)
}
