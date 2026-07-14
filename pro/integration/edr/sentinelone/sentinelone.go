// Package sentinelone implements an EDR provider backed by SentinelOne.
package sentinelone

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	edrpkg "github.com/gravitl/netmaker/pro/integration/edr"
)

const (
	providerName    = edrpkg.ProviderSentinelOne
	providerDisplay = "SentinelOne"

	agentsPath   = "/web/api/v2.1/agents"
	defaultLimit = 200
)

func init() {
	edrpkg.Register(providerName, providerDisplay, New)
	edrpkg.RegisterCapabilities(providerName, edrpkg.Capabilities{ReportsRisk: true})
}

func New(configJSON json.RawMessage) (edrpkg.Provider, error) {
	var cfg edrpkg.SentinelOneConfig
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, fmt.Errorf("invalid sentinelone config: %w", err)
	}
	if err := edrpkg.ValidateConfig(providerName, configJSON); err != nil {
		return nil, err
	}
	return &Client{
		baseURL:  strings.TrimRight(cfg.ConsoleURL, "/"),
		apiToken: cfg.APIToken,
		http:     &http.Client{Timeout: 60 * time.Second},
	}, nil
}

type Client struct {
	baseURL  string
	apiToken string
	http     *http.Client
}

func (c *Client) Name() string { return providerName }

func (c *Client) Capabilities() edrpkg.Capabilities {
	return edrpkg.Capabilities{ReportsRisk: true}
}

func (c *Client) Verify(ctx context.Context) error {
	_, err := c.listAgents(ctx, 1)
	if err != nil {
		return fmt.Errorf("sentinelone verify failed: %w", err)
	}
	return nil
}

func (c *Client) ListManagedEndpoints(ctx context.Context) ([]edrpkg.ManagedEndpoint, error) {
	var out []edrpkg.ManagedEndpoint
	for cursor := ""; ; {
		page, nextCursor, err := c.listAgentsPage(ctx, defaultLimit, cursor)
		if err != nil {
			return nil, err
		}
		for _, a := range page {
			out = append(out, normalizeAgent(a))
		}
		if nextCursor == "" || len(page) < defaultLimit {
			break
		}
		cursor = nextCursor
	}
	return out, nil
}

func (c *Client) listAgents(ctx context.Context, limit int) ([]s1Agent, error) {
	agents, _, err := c.listAgentsPage(ctx, limit, "")
	return agents, err
}

func (c *Client) listAgentsPage(ctx context.Context, limit int, cursor string) ([]s1Agent, string, error) {
	u, err := url.Parse(c.baseURL + agentsPath)
	if err != nil {
		return nil, "", err
	}
	q := u.Query()
	q.Set("limit", fmt.Sprintf("%d", limit))
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "ApiToken "+c.apiToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("sentinelone list agents: http %d", resp.StatusCode)
	}
	if readErr != nil {
		return nil, "", readErr
	}
	var page agentsResponse
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, "", err
	}
	return page.Data, page.Pagination.NextCursor, nil
}

func normalizeAgent(a s1Agent) edrpkg.ManagedEndpoint {
	last := time.Time{}
	if a.LastActiveDate != "" {
		if ts, err := time.Parse(time.RFC3339, a.LastActiveDate); err == nil {
			last = ts
		}
	}
	networkQuarantine := strings.EqualFold(strings.TrimSpace(a.NetworkStatus), "network_quarantine")
	installed := a.ID != ""
	healthy := a.IsActive && !a.Infected && !networkQuarantine
	signals := edrpkg.VendorSignals{
		AgentInstalled:   installed,
		AgentHealthy:     healthy,
		ActiveMalware:    a.Infected,
		ActiveThreats:    a.ActiveThreats > 0,
		ThreatCount:      a.ActiveThreats,
		Isolated:         networkQuarantine,
		VendorRiskLevel:  edrpkg.SentinelOneRiskFromAgent(a.Infected, networkQuarantine, a.ActiveThreats),
	}
	raw, _ := json.Marshal(a)
	return edrpkg.ManagedEndpoint{
		ProviderDeviceID: a.ID,
		SerialNumber:     a.SerialNumber,
		Hostname:         a.ComputerName,
		AgentInstalled:   installed,
		AgentHealthy:     healthy,
		ThreatCount:      a.ActiveThreats,
		ActiveThreats:    a.ActiveThreats > 0,
		Isolated:         networkQuarantine,
		RiskLevel:        edrpkg.ComputeRiskLevel(signals),
		LastSeen:         last,
		RawVendorData:    raw,
	}
}

type agentsResponse struct {
	Data       []s1Agent `json:"data"`
	Pagination struct {
		NextCursor string `json:"nextCursor"`
	} `json:"pagination"`
}

type s1Agent struct {
	ID             string `json:"id"`
	ComputerName   string `json:"computerName"`
	SerialNumber   string `json:"serialNumber"`
	IsActive       bool   `json:"isActive"`
	Infected       bool   `json:"infected"`
	ActiveThreats  int    `json:"activeThreats"`
	NetworkStatus  string `json:"networkStatus"`
	LastActiveDate string `json:"lastActiveDate"`
}
