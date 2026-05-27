package integration

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type SplunkConfig struct {
	HECEndpoint string `json:"hec_endpoint"`
	HECToken    string `json:"hec_token"`
	Index       string `json:"index"`
	Source      string `json:"source"`
	SourceType  string `json:"source_type"`
	TLSVerify   bool   `json:"tls_verify"`
}

type splunkProvider struct{}

func (s *splunkProvider) Validate(configJSON json.RawMessage) error {
	var cfg SplunkConfig
	err := json.Unmarshal(configJSON, &cfg)
	if err != nil {
		return fmt.Errorf("invalid splunk config: %w", err)
	}
	if cfg.HECEndpoint == "" {
		return fmt.Errorf("hec_endpoint is required")
	}
	if cfg.HECToken == "" {
		return fmt.Errorf("hec_token is required")
	}
	return nil
}

func (s *splunkProvider) Test(configJSON json.RawMessage) error {
	var cfg SplunkConfig
	err := json.Unmarshal(configJSON, &cfg)
	if err != nil {
		return fmt.Errorf("invalid splunk config: %w", err)
	}

	testEvent := map[string]any{
		"message": "netmaker siem integration test",
	}
	return NewSplunkSIEMClient(cfg).Export(context.Background(), []any{testEvent})
}

type SplunkSIEMClient struct {
	SplunkConfig
}

func NewSplunkSIEMClient(config SplunkConfig) *SplunkSIEMClient {
	if config.SourceType == "" {
		config.SourceType = "_json"
	}
	return &SplunkSIEMClient{SplunkConfig: config}
}

func (s *SplunkSIEMClient) Export(ctx context.Context, events []any) error {
	var buf bytes.Buffer
	for _, e := range events {
		payload := map[string]any{
			"event":      e,
			"index":      s.Index,
			"source":     s.Source,
			"sourcetype": s.SourceType,
		}
		line, _ := json.Marshal(payload)
		buf.Write(line)
		buf.WriteByte('\n')
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: !s.TLSVerify},
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.HECEndpoint, &buf)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Authorization", "Splunk "+s.HECToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reach splunk: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("splunk returned HTTP %d", resp.StatusCode)
	}
	return nil
}
