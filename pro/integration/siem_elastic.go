package integration

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ElasticConfig struct {
	Endpoint  string `json:"endpoint"`
	APIKey    string `json:"api_key"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Index     string `json:"index"`
	TLSVerify bool   `json:"tls_verify"`
}

type elasticProvider struct{}

func (e *elasticProvider) Validate(configJSON json.RawMessage) error {
	var cfg ElasticConfig
	err := json.Unmarshal(configJSON, &cfg)
	if err != nil {
		return fmt.Errorf("invalid elastic config: %w", err)
	}
	if cfg.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	if cfg.Index == "" {
		return fmt.Errorf("index is required")
	}
	if cfg.APIKey == "" && cfg.Username == "" {
		return fmt.Errorf("either api_key or username+password is required")
	}
	return nil
}

func (e *elasticProvider) Test(configJSON json.RawMessage) error {
	var cfg ElasticConfig
	json.Unmarshal(configJSON, &cfg) //nolint:errcheck // Validate must be called first

	doc := map[string]interface{}{
		"@timestamp": time.Now().UTC().Format(time.RFC3339),
		"message":    "netmaker siem integration test",
		"source":     "netmaker",
	}
	body, _ := json.Marshal(doc)

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: !cfg.TLSVerify},
		},
	}

	url := fmt.Sprintf("%s/%s/_doc", cfg.Endpoint, cfg.Index)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "ApiKey "+cfg.APIKey)
	} else {
		req.SetBasicAuth(cfg.Username, cfg.Password)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reach elasticsearch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("elasticsearch returned HTTP %d", resp.StatusCode)
	}
	return nil
}
