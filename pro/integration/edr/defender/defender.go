// Package defender implements an EDR provider backed by Microsoft Defender for
// Endpoint via the WindowsDefenderATP API.
package defender

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

var (
	providerName    = edrpkg.ProviderDefender
	providerDisplay = "Microsoft Defender for Endpoint"
	defaultPageSz   = 200

	tokenURLFmt = "https://login.microsoftonline.com/%s/oauth2/v2.0/token"
	// Defender for Endpoint tokens must target the WindowsDefenderATP resource, not Graph.
	tokenScope  = "https://api.securitycenter.microsoft.com/.default"
	machinesURL = "https://api.security.microsoft.com/api/machines"
)

func init() {
	edrpkg.Register(providerName, providerDisplay, New)
	edrpkg.RegisterCapabilities(providerName, edrpkg.Capabilities{ReportsRisk: true})
}

func New(configJSON json.RawMessage) (edrpkg.Provider, error) {
	var cfg edrpkg.DefenderConfig
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, fmt.Errorf("invalid defender config: %w", err)
	}
	if err := edrpkg.ValidateConfig(providerName, configJSON); err != nil {
		return nil, err
	}
	return &Client{
		tenantID:     cfg.TenantID,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		http:         &http.Client{Timeout: 60 * time.Second},
	}, nil
}

type Client struct {
	tenantID     string
	clientID     string
	clientSecret string
	http         *http.Client

	tokenMu       sync.Mutex
	token         string
	tokenExp      time.Time
	graphToken    string
	graphTokenExp time.Time
}

func (c *Client) Name() string { return providerName }

func (c *Client) Capabilities() edrpkg.Capabilities {
	return edrpkg.Capabilities{ReportsRisk: true}
}

func (c *Client) Verify(ctx context.Context) error {
	_, err := c.listMachines(ctx, 1)
	if err != nil {
		return fmt.Errorf("defender verify failed: %w", err)
	}
	return nil
}

func (c *Client) ListManagedEndpoints(ctx context.Context) ([]edrpkg.ManagedEndpoint, error) {
	machines, err := c.listAllMachines(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]edrpkg.ManagedEndpoint, 0, len(machines))
	for _, m := range machines {
		out = append(out, normalizeMachine(m))
	}
	return out, nil
}

func (c *Client) listAllMachines(ctx context.Context) ([]securityMachine, error) {
	tok, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	var out []securityMachine
	u := machinesURL + "?$top=" + fmt.Sprintf("%d", defaultPageSz)
	for u != "" {
		page, next, err := c.listMachinesPage(ctx, tok, u)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		u = next
	}
	return out, nil
}

func (c *Client) listMachines(ctx context.Context, top int) ([]securityMachine, error) {
	tok, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	u := machinesURL + "?$top=" + fmt.Sprintf("%d", top)
	page, _, err := c.listMachinesPage(ctx, tok, u)
	return page, err
}

func (c *Client) listMachinesPage(ctx context.Context, tok, u string) ([]securityMachine, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, "", readErr
	}
	if resp.StatusCode == http.StatusNotFound {
		// Defender returns 404 when no machines have reported in recently.
		return nil, "", nil
	}
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("defender list machines: http %d: %s", resp.StatusCode, defenderAPIError(body))
	}
	var page machinesPage
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, "", err
	}
	if page.Error.Code != "" {
		return nil, "", fmt.Errorf("defender list machines: %s", page.Error.Message)
	}
	return page.Value, page.NextLink, nil
}

func defenderAPIError(body []byte) string {
	var errBody struct {
		Error struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &errBody); err == nil {
		if msg := strings.TrimSpace(errBody.Error.Message); msg != "" {
			return msg
		}
		if code := strings.TrimSpace(errBody.Error.Code); code != "" {
			return code
		}
	}
	return strings.TrimSpace(string(body))
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
	form.Set("scope", tokenScope)
	form.Set("grant_type", "client_credentials")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf(tokenURLFmt, c.tenantID), strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	tokenBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return "", readErr
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("defender token: http %d: %s", resp.StatusCode, defenderAPIError(tokenBody))
	}
	var body tokenResponse
	if err := json.Unmarshal(tokenBody, &body); err != nil {
		return "", err
	}
	if body.AccessToken == "" {
		return "", fmt.Errorf("defender token: empty access_token")
	}
	c.token = body.AccessToken
	if body.ExpiresIn > 0 {
		c.tokenExp = time.Now().Add(time.Duration(body.ExpiresIn) * time.Second)
	} else {
		c.tokenExp = time.Now().Add(50 * time.Minute)
	}
	return c.token, nil
}

func normalizeMachine(m securityMachine) edrpkg.ManagedEndpoint {
	last, _ := time.Parse(time.RFC3339, m.LastSeen)
	onboarded := strings.EqualFold(strings.TrimSpace(m.OnboardingStatus), "onboarded")
	healthy := strings.EqualFold(strings.TrimSpace(m.HealthStatus), "active")
	signals := edrpkg.VendorSignals{
		AgentInstalled:  onboarded,
		AgentHealthy:    healthy && onboarded,
		VendorRiskLevel: edrpkg.DefenderRiskFromScore(m.RiskScore),
	}
	raw, _ := json.Marshal(m)
	return edrpkg.ManagedEndpoint{
		ProviderDeviceID: m.ID,
		Hostname:         m.ComputerDNSName,
		SerialNumber:     strings.TrimSpace(m.SerialNumber),
		EntraDeviceID:    m.AadDeviceID,
		AgentInstalled:   onboarded,
		AgentHealthy:     healthy && onboarded,
		RiskLevel:        edrpkg.ComputeRiskLevel(signals),
		LastSeen:         last,
		RawVendorData:    raw,
	}
}

type machinesPage struct {
	Value    []securityMachine `json:"value"`
	NextLink string            `json:"@odata.nextLink"`
	Error    errorBody         `json:"error"`
}

type securityMachine struct {
	ID               string `json:"id"`
	ComputerDNSName  string `json:"computerDnsName"`
	AadDeviceID      string `json:"aadDeviceId"`
	SerialNumber     string `json:"serialNumber"`
	HealthStatus     string `json:"healthStatus"`
	OnboardingStatus string `json:"onboardingStatus"`
	RiskScore        string `json:"riskScore"`
	LastSeen         string `json:"lastSeen"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
