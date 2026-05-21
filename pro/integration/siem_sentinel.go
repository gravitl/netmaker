package integration

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type SentinelConfig struct {
	WorkspaceID string `json:"workspace_id"`
	SharedKey   string `json:"shared_key"`
	LogType     string `json:"log_type"`
}

type sentinelProvider struct{}

func (s *sentinelProvider) Validate(configJSON json.RawMessage) error {
	var cfg SentinelConfig
	err := json.Unmarshal(configJSON, &cfg)
	if err != nil {
		return fmt.Errorf("invalid sentinel config: %w", err)
	}
	if cfg.WorkspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if cfg.SharedKey == "" {
		return fmt.Errorf("shared_key is required")
	}
	_, err = base64.StdEncoding.DecodeString(cfg.SharedKey)
	if err != nil {
		return fmt.Errorf("shared_key is not valid base64: %w", err)
	}
	return nil
}

func (s *sentinelProvider) Test(configJSON json.RawMessage) error {
	var cfg SentinelConfig
	err := json.Unmarshal(configJSON, &cfg)
	if err != nil {
		return fmt.Errorf("invalid sentinel config: %w", err)
	}

	testEvent := map[string]any{
		"TimeGenerated": time.Now().UTC().Format(time.RFC3339),
		"Message":       "netmaker siem integration test",
		"Source":        "netmaker",
	}
	return NewSentinelSIEMClient(cfg).Export(context.Background(), []any{testEvent})
}

type SentinelSIEMClient struct {
	SentinelConfig
}

func NewSentinelSIEMClient(config SentinelConfig) *SentinelSIEMClient {
	if config.LogType == "" {
		config.LogType = "NetmakerSIEM"
	}
	return &SentinelSIEMClient{SentinelConfig: config}
}

func (s *SentinelSIEMClient) Export(ctx context.Context, events []any) error {
	body, _ := json.Marshal(events)

	now := time.Now().UTC().Format(http.TimeFormat)
	contentLength := len(body)
	decodedKey, _ := base64.StdEncoding.DecodeString(s.SharedKey)

	stringToSign := fmt.Sprintf("POST\n%d\napplication/json\nx-ms-date:%s\n/api/logs", contentLength, now)
	mac := hmac.New(sha256.New, decodedKey)
	mac.Write([]byte(stringToSign))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	url := fmt.Sprintf("https://%s.ods.opinsights.azure.com/api/logs?api-version=2016-04-01", s.WorkspaceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Log-Type", s.LogType)
	req.Header.Set("x-ms-date", now)
	req.Header.Set("Authorization", fmt.Sprintf("SharedKey %s:%s", s.WorkspaceID, sig))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reach microsoft sentinel: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("microsoft sentinel returned HTTP %d", resp.StatusCode)
	}
	return nil
}
