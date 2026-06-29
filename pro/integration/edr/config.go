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
	ProviderWazuh       = "wazuh"
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

// WazuhConfig is stored in integrations_v1.config for Wazuh.
type WazuhConfig struct {
	SyncSettings
	ManagerURL         string `json:"manager_url"`
	Username           string `json:"username"`
	Password           string `json:"password"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
}

func ParseWazuhConfig(configJSON json.RawMessage) (WazuhConfig, error) {
	return parseWazuhConfig(configJSON)
}

func parseWazuhConfig(configJSON json.RawMessage) (WazuhConfig, error) {
	var cfg WazuhConfig
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return cfg, err
	}
	var extras struct {
		SkipTLSVerify bool `json:"skip_tls_verify"`
	}
	if err := json.Unmarshal(configJSON, &extras); err == nil && extras.SkipTLSVerify {
		cfg.InsecureSkipVerify = true
	}
	return cfg, nil
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
		url := strings.TrimSpace(cfg.BaseURL)
		if url == "" {
			return fmt.Errorf("base_url is required")
		}
		if !strings.HasPrefix(strings.ToLower(url), "https://") {
			return fmt.Errorf("base_url must use https")
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
	case ProviderWazuh:
		cfg, err := parseWazuhConfig(configJSON)
		if err != nil {
			return fmt.Errorf("invalid wazuh config: %w", err)
		}
		url := strings.TrimSpace(cfg.ManagerURL)
		if url == "" {
			return fmt.Errorf("manager_url is required")
		}
		if !strings.HasPrefix(strings.ToLower(url), "https://") {
			return fmt.Errorf("manager_url must use https")
		}
		if cfg.Username == "" {
			return fmt.Errorf("username is required")
		}
		if cfg.Password == "" {
			return fmt.Errorf("password is required")
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
	case ProviderWazuh:
		cfg, err := parseWazuhConfig(configJSON)
		if err != nil {
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
	case ProviderWazuh:
		cfg, err := parseWazuhConfig(configJSON)
		if err != nil {
			return nil, err
		}
		if cfg.Password != "" {
			cfg.Password = logic.Mask()
		}
		return json.Marshal(cfg)
	default:
		return configJSON, nil
	}
}
