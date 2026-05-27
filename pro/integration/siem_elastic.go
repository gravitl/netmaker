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
	err := json.Unmarshal(configJSON, &cfg)
	if err != nil {
		return fmt.Errorf("invalid elastic config: %w", err)
	}

	testEvent := map[string]any{
		"message": "netmaker siem integration test",
	}
	return NewElasticSIEMClient(cfg).Export(context.Background(), []any{testEvent})
}

type ElasticSIEMClient struct {
	ElasticConfig
}

func NewElasticSIEMClient(config ElasticConfig) *ElasticSIEMClient {
	return &ElasticSIEMClient{ElasticConfig: config}
}

func (e *ElasticSIEMClient) Export(ctx context.Context, events []any) error {
	metaLine, _ := json.Marshal(map[string]any{"index": map[string]any{"_index": e.Index}})
	var buf bytes.Buffer
	for _, ev := range events {
		buf.Write(metaLine)
		buf.WriteByte('\n')
		var evMap map[string]any
		data, _ := json.Marshal(ev)
		json.Unmarshal(data, &evMap)
		if _, ok := evMap["@timestamp"]; !ok {
			evMap["@timestamp"] = time.Now().UTC().Format(time.RFC3339)
		}
		docLine, _ := json.Marshal(evMap)
		buf.Write(docLine)
		buf.WriteByte('\n')
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: !e.TLSVerify},
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/_bulk", e.Endpoint), &buf)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	if e.APIKey != "" {
		req.Header.Set("Authorization", "ApiKey "+e.APIKey)
	} else {
		req.SetBasicAuth(e.Username, e.Password)
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
