// Package crowdstrike implements an EDR provider backed by CrowdStrike Falcon.
package crowdstrike

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	edrpkg "github.com/gravitl/netmaker/pro/integration/edr"
	"github.com/gravitl/netmaker/schema"
)

const (
	providerName    = edrpkg.ProviderCrowdStrike
	providerDisplay = "CrowdStrike Falcon"

	tokenPath    = "/oauth2/token"
	queryPath    = "/devices/queries/devices/v1"
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
	ids, err := c.queryDeviceIDs(ctx, tok, "")
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

// LookupBySerial resolves a Falcon host by serial_number using the devices
// query filter API instead of listing the full fleet.
func (c *Client) LookupBySerial(ctx context.Context, serial string) (edrpkg.ManagedEndpoint, error) {
	serial = strings.TrimSpace(serial)
	if serial == "" {
		return edrpkg.ManagedEndpoint{}, edrpkg.ErrDeviceNotFoundInEDR
	}
	ep, _, err := c.lookupForSerial(ctx, serial)
	return ep, err
}

// LookupForHost resolves a Falcon endpoint by serial_number only.
func (c *Client) LookupForHost(ctx context.Context, h schema.Host) (edrpkg.ManagedEndpoint, string, error) {
	serial := strings.TrimSpace(h.SerialNumber)
	if serial == "" {
		return edrpkg.ManagedEndpoint{}, "", edrpkg.ErrDeviceNotFoundInEDR
	}
	return c.lookupForSerial(ctx, serial)
}

func (c *Client) lookupForSerial(ctx context.Context, serial string) (edrpkg.ManagedEndpoint, string, error) {
	tok, err := c.accessToken(ctx)
	if err != nil {
		return edrpkg.ManagedEndpoint{}, "", err
	}
	deviceID, err := c.searchDeviceBySerial(ctx, tok, serial)
	if err != nil {
		if errors.Is(err, edrpkg.ErrDeviceNotFoundInEDR) {
			return edrpkg.ManagedEndpoint{}, "", err
		}
		return edrpkg.ManagedEndpoint{}, "", err
	}
	devices, err := c.fetchDevices(ctx, tok, []string{deviceID})
	if err != nil {
		return edrpkg.ManagedEndpoint{}, "", err
	}
	if len(devices) == 0 {
		return edrpkg.ManagedEndpoint{}, "", edrpkg.ErrDeviceNotFoundInEDR
	}
	return normalizeDevice(devices[0]), schema.EDRMatchSerialNumber, nil
}

func (c *Client) searchDeviceBySerial(ctx context.Context, token, serial string) (string, error) {
	escapedSerial := strings.ReplaceAll(serial, "'", "''")
	filter := url.QueryEscape(fmt.Sprintf("serial_number:'%s'", escapedSerial))
	return c.searchDeviceByFilter(ctx, token, filter)
}

func (c *Client) searchDeviceByFilter(ctx context.Context, token, encodedFilter string) (string, error) {
	req, err := http.NewRequestWithContext(ctx,
		http.MethodGet,
		fmt.Sprintf("%s/devices/queries/devices/v1?filter=%s", c.baseURL, encodedFilter),
		nil,
	)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return "", readErr
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("crowdstrike query devices: http %d", resp.StatusCode)
	}

	var result queryResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if len(result.Resources) == 0 {
		return "", fmt.Errorf("%w", edrpkg.ErrDeviceNotFoundInEDR)
	}

	return result.Resources[0], nil
}

func (c *Client) queryDeviceIDs(ctx context.Context, tok, filter string) ([]string, error) {
	var ids []string
	for offset := ""; ; {
		u := c.baseURL + queryPath + "?limit=" + fmt.Sprintf("%d", defaultLimit)
		if filter != "" {
			u += "&filter=" + url.QueryEscape(filter)
		}
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
		offset = page.Meta.Pagination.Offset.String()
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
	contained := edrpkg.CrowdStrikeContainedFromStatus(d.Status)
	installed := strings.TrimSpace(d.SerialNumber) != ""
	healthy := installed && edrpkg.CrowdStrikeHealthyFromStatus(d.Status)
	signals := edrpkg.VendorSignals{
		AgentInstalled:  installed,
		AgentHealthy:    healthy,
		Contained:       contained,
		VendorRiskLevel: edrpkg.CrowdStrikeRiskFromStatus(d.Status),
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
			Offset paginationOffset `json:"offset"`
			Next   string           `json:"next"`
		} `json:"pagination"`
	} `json:"meta"`
}

// paginationOffset accepts CrowdStrike offset as either a string or number.
type paginationOffset string

func (p *paginationOffset) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*p = paginationOffset(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		*p = paginationOffset(n.String())
		return nil
	}
	var i int
	if err := json.Unmarshal(data, &i); err == nil {
		*p = paginationOffset(strconv.Itoa(i))
		return nil
	}
	return fmt.Errorf("invalid pagination offset: %s", string(data))
}

func (p paginationOffset) String() string {
	return string(p)
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
