package integration

import (
	"bytes"
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

	payload := map[string]interface{}{
		"event": map[string]interface{}{
			"message": "netmaker siem integration test",
			"source":  "netmaker",
		},
		"index":      cfg.Index,
		"source":     cfg.Source,
		"sourcetype": cfg.SourceType,
	}
	body, _ := json.Marshal(payload)

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: !cfg.TLSVerify},
		},
	}
	req, err := http.NewRequest(http.MethodPost, cfg.HECEndpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Authorization", "Splunk "+cfg.HECToken)
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
