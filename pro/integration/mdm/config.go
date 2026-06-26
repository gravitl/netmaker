package mdm

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gravitl/netmaker/logic"
)

const (
	ProviderIntune    = "intune"
	ProviderJamf      = "jamf"
	ProviderJumpCloud = "jumpcloud"
	ProviderIru       = "iru"
)

// SyncSettings are shared across MDM provider configs.
type SyncSettings struct {
	SyncEnabled         bool `json:"sync_enabled"`
	SyncIntervalMinutes int  `json:"sync_interval_minutes"`
}

// IntuneConfig is stored in integrations_v1.config for the intune provider.
type IntuneConfig struct {
	SyncSettings
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	TenantID     string `json:"tenant_id"`
}

// JamfConfig is stored in integrations_v1.config for the jamf provider.
//
// ComplianceVendors optionally limits device-trust evaluation to named
// complianceVendor values from Jamf Conditional Access (e.g. "Jamf", "Intune").
// When empty, every applicable compliance record for the device must be COMPLIANT.
type JamfConfig struct {
	SyncSettings
	ClientID            string   `json:"client_id"`
	ClientSecret        string   `json:"client_secret"`
	BaseURL             string   `json:"base_url"`
	ComplianceVendors   []string `json:"compliance_vendors,omitempty"`
}

// JumpCloudConfig is stored in integrations_v1.config for the jumpcloud provider.
// Auth uses a JumpCloud service account (client_id + client_secret) against
// admin-oauth.id.jumpcloud.com with Basic auth. BaseURL defaults to https://console.jumpcloud.com.
//
// CompliancePolicyIDs optionally limits device-trust evaluation to specific JumpCloud
// policy object IDs. When empty, all policy statuses returned for each system must pass.
type JumpCloudConfig struct {
	SyncSettings
	ClientID            string   `json:"client_id"`
	ClientSecret        string   `json:"client_secret"`
	BaseURL             string   `json:"base_url"`
	CompliancePolicyIDs []string `json:"compliance_policy_ids,omitempty"`
}

// IruConfig is stored in integrations_v1.config for the iru provider (Iru Endpoint
// Management, formerly Kandji). APIURL is the tenant API hostname from Settings
// (e.g. https://acme.api.iru.com or https://acme.api.kandji.io).
//
// ComplianceLibraryItemIDs optionally limits compliance evaluation to specific
// library item IDs from GET /api/v1/devices/{device_id}/status. When empty,
// all parameters and library items must report a passing status (PASS,
// REMEDIATED/EXCLUDED/WARNING for parameters; PASS/success/EXCLUDED/AVAILABLE
// for library items).
type IruConfig struct {
	SyncSettings
	APIURL                   string   `json:"api_url"`
	APIToken                 string   `json:"api_token"`
	ComplianceLibraryItemIDs []string `json:"compliance_library_item_ids,omitempty"`
}

// ValidateConfig validates provider config JSON for the given provider id.
func ValidateConfig(providerID string, configJSON json.RawMessage) error {
	switch providerID {
	case ProviderIntune:
		var cfg IntuneConfig
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return fmt.Errorf("invalid intune config: %w", err)
		}
		if cfg.TenantID == "" {
			return fmt.Errorf("tenant_id is required")
		}
		if cfg.ClientID == "" {
			return fmt.Errorf("client_id is required")
		}
		if cfg.ClientSecret == "" {
			return fmt.Errorf("client_secret is required")
		}
		if cfg.SyncIntervalMinutes < 0 {
			return fmt.Errorf("sync_interval_minutes must be >= 0")
		}
		return nil
	case ProviderJamf:
		var cfg JamfConfig
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return fmt.Errorf("invalid jamf config: %w", err)
		}
		if strings.TrimSpace(cfg.BaseURL) == "" {
			return fmt.Errorf("base_url is required")
		}
		if cfg.ClientID == "" {
			return fmt.Errorf("client_id is required")
		}
		if cfg.ClientSecret == "" {
			return fmt.Errorf("client_secret is required")
		}
		if cfg.SyncIntervalMinutes < 0 {
			return fmt.Errorf("sync_interval_minutes must be >= 0")
		}
		return nil
	case ProviderJumpCloud:
		var cfg JumpCloudConfig
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return fmt.Errorf("invalid jumpcloud config: %w", err)
		}
		if cfg.ClientID == "" {
			return fmt.Errorf("client_id is required")
		}
		if cfg.ClientSecret == "" {
			return fmt.Errorf("client_secret is required")
		}
		if cfg.SyncIntervalMinutes < 0 {
			return fmt.Errorf("sync_interval_minutes must be >= 0")
		}
		return nil
	case ProviderIru:
		var cfg IruConfig
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return fmt.Errorf("invalid iru config: %w", err)
		}
		apiURL := strings.TrimSpace(cfg.APIURL)
		if apiURL == "" {
			return fmt.Errorf("api_url is required")
		}
		if !strings.HasPrefix(strings.ToLower(apiURL), "https://") {
			return fmt.Errorf("api_url must use https")
		}
		if cfg.APIToken == "" {
			return fmt.Errorf("api_token is required")
		}
		if cfg.SyncIntervalMinutes < 0 {
			return fmt.Errorf("sync_interval_minutes must be >= 0")
		}
		return nil
	default:
		return fmt.Errorf("unknown mdm provider %q", providerID)
	}
}

// ParseSyncSettings extracts sync settings from stored integration config.
func ParseSyncSettings(providerID string, configJSON json.RawMessage) (SyncSettings, error) {
	switch providerID {
	case ProviderIntune:
		var cfg IntuneConfig
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return SyncSettings{}, err
		}
		return cfg.SyncSettings, nil
	case ProviderJamf:
		var cfg JamfConfig
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return SyncSettings{}, err
		}
		return cfg.SyncSettings, nil
	case ProviderJumpCloud:
		var cfg JumpCloudConfig
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return SyncSettings{}, err
		}
		return cfg.SyncSettings, nil
	case ProviderIru:
		var cfg IruConfig
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return SyncSettings{}, err
		}
		return cfg.SyncSettings, nil
	default:
		return SyncSettings{}, fmt.Errorf("unknown mdm provider %q", providerID)
	}
}

// RedactConfig returns config JSON with secrets masked for API responses.
func RedactConfig(providerID string, configJSON json.RawMessage) (json.RawMessage, error) {
	switch providerID {
	case ProviderIntune:
		var cfg IntuneConfig
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return nil, err
		}
		if cfg.ClientSecret != "" {
			cfg.ClientSecret = logic.Mask()
		}
		return json.Marshal(cfg)
	case ProviderJamf:
		var cfg JamfConfig
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return nil, err
		}
		if cfg.ClientSecret != "" {
			cfg.ClientSecret = logic.Mask()
		}
		return json.Marshal(cfg)
	case ProviderJumpCloud:
		var cfg JumpCloudConfig
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return nil, err
		}
		if cfg.ClientSecret != "" {
			cfg.ClientSecret = logic.Mask()
		}
		return json.Marshal(cfg)
	case ProviderIru:
		var cfg IruConfig
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return nil, err
		}
		if cfg.APIToken != "" {
			cfg.APIToken = logic.Mask()
		}
		return json.Marshal(cfg)
	default:
		return configJSON, nil
	}
}
