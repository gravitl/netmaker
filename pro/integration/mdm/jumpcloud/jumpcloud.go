// Package jumpcloud implements an MDM provider backed by JumpCloud. Self-registers
// with pro/integration/mdm in init().
package jumpcloud

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	mdmpkg "github.com/gravitl/netmaker/pro/integration/mdm"
)

const (
	providerName    = mdmpkg.ProviderJumpCloud
	providerDisplay = "JumpCloud"

	tokenURL          = "https://admin-oauth.id.jumpcloud.com/oauth2/token"
	defaultBaseURL    = "https://console.jumpcloud.com"
	systemsListPath   = "/api/systems"
	defaultPageSize   = 100
	tokenScope        = "api"
	maxPolicyFetches  = 8
)

func init() {
	mdmpkg.Register(providerName, providerDisplay, New)
	mdmpkg.RegisterCapabilities(providerName, mdmpkg.Capabilities{ReportsCompliant: true})
}

// New builds a JumpCloud provider from integration config JSON.
func New(configJSON json.RawMessage) (mdmpkg.Provider, error) {
	var cfg mdmpkg.JumpCloudConfig
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, fmt.Errorf("invalid jumpcloud config: %w", err)
	}
	if err := mdmpkg.ValidateConfig(providerName, configJSON); err != nil {
		return nil, err
	}
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		baseURL:             baseURL,
		clientID:            cfg.ClientID,
		clientSecret:        cfg.ClientSecret,
		compliancePolicyIDs: policyIDSet(cfg.CompliancePolicyIDs),
		http:                &http.Client{Timeout: 60 * time.Second},
	}, nil
}

// Client implements mdmpkg.Provider against JumpCloud.
type Client struct {
	baseURL             string
	clientID            string
	clientSecret        string
	compliancePolicyIDs map[string]struct{}
	http                *http.Client

	tokenMu  sync.Mutex
	token    string
	tokenExp time.Time
}

func (c *Client) Name() string { return providerName }

func (c *Client) Capabilities() mdmpkg.Capabilities {
	return mdmpkg.Capabilities{ReportsCompliant: true}
}

func (c *Client) Verify(ctx context.Context) error {
	tok, err := c.accessToken(ctx)
	if err != nil {
		return err
	}
	_, err = c.listSystems(ctx, tok, 0, 1)
	if err != nil {
		return fmt.Errorf("jumpcloud verify failed: %w", err)
	}
	return nil
}

func (c *Client) ListManagedDevices(ctx context.Context) ([]mdmpkg.ManagedDevice, error) {
	tok, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	var systems []jumpcloudSystem
	for skip := 0; ; skip += defaultPageSize {
		page, err := c.listSystems(ctx, tok, skip, defaultPageSize)
		if err != nil {
			return nil, err
		}
		systems = append(systems, page...)
		if len(page) < defaultPageSize {
			break
		}
	}
	complianceByID, err := c.fetchDeviceTrustCompliance(ctx, tok, systems)
	if err != nil {
		return nil, err
	}
	out := make([]mdmpkg.ManagedDevice, 0, len(systems))
	for _, s := range systems {
		compliant, ok := complianceByID[s.ID]
		if !ok {
			compliant = false
		}
		out = append(out, normalize(s, compliant))
	}
	return out, nil
}

func (c *Client) fetchDeviceTrustCompliance(ctx context.Context, tok string, systems []jumpcloudSystem) (map[string]bool, error) {
	out := make(map[string]bool, len(systems))
	if len(systems) == 0 {
		return out, nil
	}
	sem := make(chan struct{}, maxPolicyFetches)
	type result struct {
		id        string
		compliant bool
		err       error
	}
	ch := make(chan result, len(systems))
	for _, s := range systems {
		s := s
		go func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			statuses, err := c.listSystemPolicyStatuses(ctx, tok, s.ID)
			if err != nil {
				ch <- result{id: s.ID, err: err}
				return
			}
			ch <- result{
				id:        s.ID,
				compliant: deviceTrustCompliant(statuses, c.compliancePolicyIDs),
			}
		}()
	}
	var firstErr error
	success := 0
	for range systems {
		r := <-ch
		if r.err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("system %s policy statuses: %w", r.id, r.err)
			}
			out[r.id] = false
			continue
		}
		out[r.id] = r.compliant
		success++
	}
	if success == 0 && firstErr != nil {
		return out, firstErr
	}
	return out, nil
}

func (c *Client) listSystemPolicyStatuses(ctx context.Context, tok, systemID string) ([]policyResult, error) {
	u := c.baseURL + systemPolicyStatusesPath + url.PathEscape(systemID) + "/policystatuses"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	var results []policyResult
	if err := json.Unmarshal(respBody, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (c *Client) listSystems(ctx context.Context, tok string, skip, limit int) ([]jumpcloudSystem, error) {
	u, err := url.Parse(c.baseURL + systemsListPath)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("skip", fmt.Sprintf("%d", skip))
	q.Set("limit", fmt.Sprintf("%d", limit))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("jumpcloud systems list: http %d", resp.StatusCode)
	}

	var wrapped systemsListResponse
	if err := json.Unmarshal(respBody, &wrapped); err == nil {
		return wrapped.Results, nil
	}
	// Some responses may return a bare array.
	var systems []jumpcloudSystem
	if err := json.Unmarshal(respBody, &systems); err != nil {
		return nil, err
	}
	return systems, nil
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.token != "" && time.Until(c.tokenExp) > time.Minute {
		return c.token, nil
	}
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("scope", tokenScope)
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		tokenURL,
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set(
		"Authorization",
		"Basic "+base64.StdEncoding.EncodeToString([]byte(c.clientID+":"+c.clientSecret)),
	)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", errors.New("jumpcloud token: " + err.Error())
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("jumpcloud token: http %d", resp.StatusCode)
	}
	var body tokenResponse
	if err := json.Unmarshal(respBody, &body); err != nil {
		return "", err
	}
	if body.AccessToken == "" {
		if body.Error != "" {
			return "", fmt.Errorf("jumpcloud token: %s: %s", body.Error, body.ErrorDescription)
		}
		return "", errors.New("jumpcloud token: empty response")
	}
	c.token = body.AccessToken
	if body.ExpiresIn > 0 {
		c.tokenExp = time.Now().Add(time.Duration(body.ExpiresIn) * time.Second)
	} else {
		c.tokenExp = time.Now().Add(50 * time.Minute)
	}
	return c.token, nil
}

func normalize(s jumpcloudSystem, compliant bool) mdmpkg.ManagedDevice {
	name := s.DisplayName
	if name == "" {
		name = s.ID
	}
	last := time.Time{}
	for _, raw := range []string{s.LastContact, s.Modified, s.Created} {
		if raw == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			last = t
			break
		}
	}
	email := ""
	if s.PrimarySystemUser != nil {
		email = s.PrimarySystemUser.Email
	}
	return mdmpkg.ManagedDevice{
		ProviderDeviceID:  s.ID,
		SerialNumber:      s.SerialNumber,
		HardwareUUID:      s.HardwareUUID,
		DeviceName:        name,
		UserPrincipalName: email,
		Enrolled:          s.Active,
		Compliant:         compliant,
		LastSeenAt:        last,
	}
}

type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type systemsListResponse struct {
	Results    []jumpcloudSystem `json:"results"`
	TotalCount int               `json:"totalCount"`
}

type jumpcloudSystem struct {
	ID          string             `json:"id"`
	DisplayName string             `json:"displayName"`
	Hostname    string             `json:"hostname"`
	SerialNumber string            `json:"serialNumber"`
	HardwareUUID string            `json:"hardwareUuid"`
	Active      bool               `json:"active"`
	LastContact string             `json:"lastContact"`
	Created     string             `json:"created"`
	Modified    string             `json:"modified"`
	PrimarySystemUser *primarySystemUser `json:"primarySystemUser"`
}

type primarySystemUser struct {
	Email string `json:"email"`
}
