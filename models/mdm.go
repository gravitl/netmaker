package models

import "time"

// MDMProviderType indicates the type of MDM integration (e.g. Intune, Jamf)
type MDMProviderType string

const (
	MDMProviderIntune MDMProviderType = "intune"
	MDMProviderJamf   MDMProviderType = "jamf"
	MDMProviderKandji MDMProviderType = "kandji"
)

// MDMConfig represents the configuration for an MDM provider integration in the database
type MDMConfig struct {
	ID       string          `json:"id" bson:"_id"`
	Provider MDMProviderType `json:"provider" bson:"provider"`
	Enabled  bool            `json:"enabled" bson:"enabled"`

	// API Configuration Details
	BaseURL      string `json:"base_url" bson:"base_url"`
	TenantID     string `json:"tenant_id,omitempty" bson:"tenant_id,omitempty"` // Used by Entra/Intune
	ClientID     string `json:"client_id,omitempty" bson:"client_id,omitempty"` // Used for OAuth/API access
	ClientSecret string `json:"client_secret,omitempty" bson:"client_secret,omitempty"`

	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

// DevicePosture represents the compliance status returned by an MDM provider
type DevicePosture struct {
	IsCompliant   bool      `json:"is_compliant"`
	LastCheckTime time.Time `json:"last_check_time"`
	Reason        string    `json:"reason,omitempty"` // Explanation if non-compliant
}

// MDMProvider is the abstraction interface that all specific MDM API clients must implement
type MDMProvider interface {
	// Initialize configures the provider client with the MDMConfig
	Initialize(config MDMConfig) error
	
	// CheckCompliance queries the upstream MDM to check if the Netmaker host is compliant
	CheckCompliance(host Host) (DevicePosture, error)
}
