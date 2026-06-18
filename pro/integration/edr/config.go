package edr

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gravitl/netmaker/logic"
)

const (
	ProviderDefender    = "defender"
	ProviderCrowdStrike = "crowdstrike"
	ProviderSentinelOne = "sentinelone"
)

// SyncSettings are shared across EDR provider configs.
type SyncSettings struct {
	SyncEnabled         bool `json:"sync_enabled"`
	SyncIntervalMinutes int  `json:"sync_interval_minutes"`
}

// DefenderConfig is stored in integrations_v1.config for Microsoft Defender for Endpoint.
type DefenderConfig struct {
	SyncSettings
	TenantID     string `json:"tenant_id"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// CrowdStrikeConfig is stored in integrations_v1.config for CrowdStrike Falcon.
type CrowdStrikeConfig struct {
	SyncSettings
	BaseURL      string `json:"base_url"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// SentinelOneConfig is stored in integrations_v1.config for SentinelOne.
type SentinelOneConfig struct {
	SyncSettings
	ConsoleURL string `json:"console_url"`
	APIToken   string `json:"api_token"`
}

func ValidateConfig(providerID string, configJSON json.RawMessage) error {
	switch providerID {
	case ProviderDefender:
		var cfg DefenderConfig
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return fmt.Errorf("invalid defender config: %w", err)
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
	case ProviderCrowdStrike:
		var cfg CrowdStrikeConfig
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return fmt.Errorf("invalid crowdstrike config: %w", err)
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
	case ProviderSentinelOne:
		var cfg SentinelOneConfig
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return fmt.Errorf("invalid sentinelone config: %w", err)
		}
		url := strings.TrimSpace(cfg.ConsoleURL)
		if url == "" {
			return fmt.Errorf("console_url is required")
		}
		if !strings.HasPrefix(strings.ToLower(url), "https://") {
			return fmt.Errorf("console_url must use https")
		}
		if cfg.APIToken == "" {
			return fmt.Errorf("api_token is required")
		}
		if cfg.SyncIntervalMinutes < 0 {
			return fmt.Errorf("sync_interval_minutes must be >= 0")
		}
		return nil
	default:
		return fmt.Errorf("unknown edr provider %q", providerID)
	}
}

func ParseSyncSettings(providerID string, configJSON json.RawMessage) (SyncSettings, error) {
	switch providerID {
	case ProviderDefender:
		var cfg DefenderConfig
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return SyncSettings{}, err
		}
		return cfg.SyncSettings, nil
	case ProviderCrowdStrike:
		var cfg CrowdStrikeConfig
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return SyncSettings{}, err
		}
		return cfg.SyncSettings, nil
	case ProviderSentinelOne:
		var cfg SentinelOneConfig
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return SyncSettings{}, err
		}
		return cfg.SyncSettings, nil
	default:
		return SyncSettings{}, fmt.Errorf("unknown edr provider %q", providerID)
	}
}

func RedactConfig(providerID string, configJSON json.RawMessage) (json.RawMessage, error) {
	switch providerID {
	case ProviderDefender:
		var cfg DefenderConfig
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return nil, err
		}
		if cfg.ClientSecret != "" {
			cfg.ClientSecret = logic.Mask()
		}
		return json.Marshal(cfg)
	case ProviderCrowdStrike:
		var cfg CrowdStrikeConfig
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return nil, err
		}
		if cfg.ClientSecret != "" {
			cfg.ClientSecret = logic.Mask()
		}
		return json.Marshal(cfg)
	case ProviderSentinelOne:
		var cfg SentinelOneConfig
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
