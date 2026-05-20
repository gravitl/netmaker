package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type DatadogConfig struct {
	APIKey  string   `json:"api_key"`
	Site    string   `json:"site"`
	Service string   `json:"service"`
	Tags    []string `json:"tags"`
}

type datadogProvider struct{}

func (d *datadogProvider) Validate(configJSON json.RawMessage) error {
	var cfg DatadogConfig
	err := json.Unmarshal(configJSON, &cfg)
	if err != nil {
		return fmt.Errorf("invalid datadog config: %w", err)
	}
	if cfg.APIKey == "" {
		return fmt.Errorf("api_key is required")
	}
	return nil
}

func (d *datadogProvider) Test(configJSON json.RawMessage) error {
	var cfg DatadogConfig
	err := json.Unmarshal(configJSON, &cfg)
	if err != nil {
		return fmt.Errorf("invalid datadog config: %w", err)
	}
	if cfg.Site == "" {
		cfg.Site = "datadoghq.com"
	}

	logs := []map[string]interface{}{
		{
			"ddsource": "netmaker",
			"ddtags":   strings.Join(cfg.Tags, ","),
			"hostname": "netmaker",
			"message":  "netmaker siem integration test",
			"service":  cfg.Service,
		},
	}
	body, _ := json.Marshal(logs)

	url := fmt.Sprintf("https://http-intake.logs.%s/api/v2/logs", cfg.Site)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("DD-API-KEY", cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reach datadog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("datadog returned HTTP %d", resp.StatusCode)
	}
	return nil
}
