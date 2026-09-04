package siem

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/go-retryablehttp"
)

type DatadogConfig struct {
	APIKey  string   `json:"api_key"`
	Site    string   `json:"site"`
	Service string   `json:"service"`
	Tags    []string `json:"tags"`
}

var validSites = map[string]bool{
	"datadoghq.com":     true,
	"us3.datadoghq.com": true,
	"us5.datadoghq.com": true,
	"datadoghq.eu":      true,
	"ddog-gov.com":      true,
	"us2.ddog-gov.com":  true,
	"ap1.datadoghq.com": true,
	"ap2.datadoghq.com": true,
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
	if cfg.Site != "" {
		if !validSites[cfg.Site] {
			return fmt.Errorf("invalid site")
		}
	}
	return nil
}

func (d *datadogProvider) Test(configJSON json.RawMessage) error {
	var cfg DatadogConfig
	err := json.Unmarshal(configJSON, &cfg)
	if err != nil {
		return fmt.Errorf("invalid datadog config: %w", err)
	}

	testEvent := map[string]any{
		"message": "netmaker siem integration test",
	}
	return NewDatadogClient(cfg).Export(context.Background(), []any{testEvent})
}

type DatadogClient struct {
	DatadogConfig
}

func NewDatadogClient(config DatadogConfig) *DatadogClient {
	if config.Site == "" {
		config.Site = "datadoghq.com"
	}
	return &DatadogClient{DatadogConfig: config}
}

type datadogLogItem struct {
	Message  string `json:"message"`
	DDSource string `json:"ddsource"`
	Service  string `json:"service,omitempty"`
	DDTags   string `json:"ddtags,omitempty"`
}

func (d *DatadogClient) Export(ctx context.Context, events []any) error {
	items := make([]datadogLogItem, 0, len(events))
	for _, e := range events {
		msg, _ := json.Marshal(e)
		item := datadogLogItem{
			Message:  string(msg),
			DDSource: "netmaker",
		}
		if d.Service != "" {
			item.Service = d.Service
		}
		if len(d.Tags) > 0 {
			item.DDTags = strings.Join(d.Tags, ",")
		}
		items = append(items, item)
	}

	body, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("failed to marshal log items: %w", err)
	}

	url := fmt.Sprintf("https://http-intake.logs.%s/api/v2/logs", d.Site)
	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("DD-API-KEY", d.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := retryablehttp.NewClient()
	client.Logger = nil

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to export to datadog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("datadog returned status %d", resp.StatusCode)
	}
	return nil
}

func DatadogProvider() *datadogProvider { return &datadogProvider{} }
