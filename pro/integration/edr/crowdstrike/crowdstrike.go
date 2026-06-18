// Package crowdstrike implements an EDR provider backed by CrowdStrike Falcon.
package crowdstrike

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	edrpkg "github.com/gravitl/netmaker/pro/integration/edr"
)

const (
	providerName    = edrpkg.ProviderCrowdStrike
	providerDisplay = "CrowdStrike Falcon"

	tokenPath   = "/oauth2/token"
	queryPath   = "/devices/queries/devices/v1"
	entitiesPath = "/devices/entities/devices/v2"
	defaultLimit = 200
)

func init() {
	edrpkg.Register(providerName, providerDisplay, New)
	edrpkg.RegisterCapabilities(providerName, edrpkg.Capabilities{ReportsRisk: true})
}

func New(configJSON json.RawMessage) (edrpkg.Provider, error) {
	var cfg edrpkg.CrowdStrikeConfig
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, fmt.Errorf("invalid crowdstrike config: %w", err)
	}
	if err := edrpkg.ValidateConfig(providerName, configJSON); err != nil {
		return nil, err
	}
	return &Client{
		baseURL:      strings.TrimRight(cfg.BaseURL, "/"),
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		http:         &http.Client{Timeout: 60 * time.Second},
	}, nil
}

type Client struct {
	baseURL      string
	clientID     string
	clientSecret string
	http         *http.Client

	tokenMu  sync.Mutex
	token    string
	tokenExp time.Time
}

func (c *Client) Name() string { return providerName }

func (c *Client) Capabilities() edrpkg.Capabilities {
	return edrpkg.Capabilities{ReportsRisk: true}
}

func (c *Client) Verify(ctx context.Context) error {
	tok, err := c.accessToken(ctx)
	if err != nil {
		return err
	}
	u := c.baseURL + queryPath + "?limit=1"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("crowdstrike verify failed: http %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) ListManagedEndpoints(ctx context.Context) ([]edrpkg.ManagedEndpoint, error) {
	tok, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	ids, err := c.queryDeviceIDs(ctx, tok)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	devices, err := c.fetchDevices(ctx, tok, ids)
	if err != nil {
		return nil, err
	}
	out := make([]edrpkg.ManagedEndpoint, 0, len(devices))
	for _, d := range devices {
		out = append(out, normalizeDevice(d))
	}
	return out, nil
}

func (c *Client) queryDeviceIDs(ctx context.Context, tok string) ([]string, error) {
	var ids []string
	for offset := ""; ; {
		u := c.baseURL + queryPath + "?limit=" + fmt.Sprintf("%d", defaultLimit)
		if offset != "" {
			u += "&offset=" + url.QueryEscape(offset)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("crowdstrike query devices: http %d", resp.StatusCode)
		}
		if readErr != nil {
			return nil, readErr
		}
		var page queryResponse
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, err
		}
		ids = append(ids, page.Resources...)
		if page.Meta.Pagination.Next == "" {
			break
		}
		offset = page.Meta.Pagination.Offset
	}
	return ids, nil
}

func (c *Client) fetchDevices(ctx context.Context, tok string, ids []string) ([]falconDevice, error) {
	var out []falconDevice
	for i := 0; i < len(ids); i += 100 {
		end := i + 100
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[i:end]
		u := c.baseURL + entitiesPath + "?ids=" + url.QueryEscape(strings.Join(chunk, ","))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("crowdstrike device entities: http %d", resp.StatusCode)
		}
		if readErr != nil {
			return nil, readErr
		}
		var page entitiesResponse
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, err
		}
		out = append(out, page.Resources...)
	}
	return out, nil
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.token != "" && time.Until(c.tokenExp) > time.Minute {
		return c.token, nil
	}
	form := url.Values{}
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+tokenPath, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("crowdstrike token: http %d", resp.StatusCode)
	}
	var body tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	if body.AccessToken == "" {
		return "", fmt.Errorf("crowdstrike token: empty access_token")
	}
	c.token = body.AccessToken
	if body.ExpiresIn > 0 {
		c.tokenExp = time.Now().Add(time.Duration(body.ExpiresIn) * time.Second)
	} else {
		c.tokenExp = time.Now().Add(30 * time.Minute)
	}
	return c.token, nil
}

func normalizeDevice(d falconDevice) edrpkg.ManagedEndpoint {
	last := time.Time{}
	if d.LastSeen != "" {
		if ts, err := time.Parse(time.RFC3339, d.LastSeen); err == nil {
			last = ts
		}
	}
	contained := strings.EqualFold(strings.TrimSpace(d.Status), "containment")
	installed := d.SerialNumber != "" || d.Hostname != ""
	healthy := installed && !contained && strings.TrimSpace(d.Status) != ""
	signals := edrpkg.VendorSignals{
		AgentInstalled:  installed,
		AgentHealthy:  healthy,
		Contained:     contained,
		VendorRiskLevel: edrpkg.CrowdStrikeRiskFromStatus(d.Status, contained),
	}
	raw, _ := json.Marshal(d)
	return edrpkg.ManagedEndpoint{
		ProviderDeviceID: d.DeviceID,
		SerialNumber:     d.SerialNumber,
		Hostname:         d.Hostname,
		AgentInstalled:   installed,
		AgentHealthy:     healthy,
		Contained:        contained,
		RiskLevel:        edrpkg.ComputeRiskLevel(signals),
		LastSeen:         last,
		RawVendorData:    raw,
	}
}

type queryResponse struct {
	Resources []string `json:"resources"`
	Meta      struct {
		Pagination struct {
			Offset string `json:"offset"`
			Next   string `json:"next"`
		} `json:"pagination"`
	} `json:"meta"`
}

type entitiesResponse struct {
	Resources []falconDevice `json:"resources"`
}

type falconDevice struct {
	DeviceID     string `json:"device_id"`
	Hostname     string `json:"hostname"`
	SerialNumber string `json:"serial_number"`
	Status       string `json:"status"`
	LastSeen     string `json:"last_seen"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}
