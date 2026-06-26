// Package wazuh implements an EDR provider backed by the Wazuh server API.
package wazuh

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	edrpkg "github.com/gravitl/netmaker/pro/integration/edr"
	"github.com/gravitl/netmaker/schema"
)

const (
	providerName    = edrpkg.ProviderWazuh
	providerDisplay = "Wazuh"

	authPath     = "/security/user/authenticate"
	agentsPath   = "/agents"
	hardwarePath = "/experimental/syscollector/hardware"
	defaultLimit = 500
)

func init() {
	edrpkg.Register(providerName, providerDisplay, New)
	edrpkg.RegisterCapabilities(providerName, edrpkg.Capabilities{ReportsRisk: true})
}

func New(configJSON json.RawMessage) (edrpkg.Provider, error) {
	cfg, err := edrpkg.ParseWazuhConfig(configJSON)
	if err != nil {
		return nil, fmt.Errorf("invalid wazuh config: %w", err)
	}
	if err := edrpkg.ValidateConfig(providerName, configJSON); err != nil {
		return nil, err
	}
	httpClient := &http.Client{Timeout: 60 * time.Second}
	if cfg.InsecureSkipVerify {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		}
	}
	return &Client{
		baseURL:  strings.TrimRight(cfg.ManagerURL, "/"),
		username: cfg.Username,
		password: cfg.Password,
		http:     httpClient,
	}, nil
}

type Client struct {
	baseURL  string
	username string
	password string
	http     *http.Client

	tokenMu  sync.Mutex
	token    string
	tokenExp time.Time
}

func (c *Client) Name() string { return providerName }

func (c *Client) Capabilities() edrpkg.Capabilities {
	return edrpkg.Capabilities{ReportsRisk: true}
}

func (c *Client) Verify(ctx context.Context) error {
	if _, err := c.accessToken(ctx); err != nil {
		return fmt.Errorf("wazuh verify failed: %w", err)
	}
	return nil
}

func (c *Client) ListManagedEndpoints(ctx context.Context) ([]edrpkg.ManagedEndpoint, error) {
	agents, err := c.listAllAgents(ctx)
	if err != nil {
		return nil, err
	}
	serialByAgent, err := c.listHardwareSerials(ctx)
	if err != nil {
		serialByAgent = map[string]string{}
	}
	out := make([]edrpkg.ManagedEndpoint, 0, len(agents))
	for _, a := range agents {
		if serial := serialByAgent[a.ID]; serial != "" {
			a.SerialNumber = serial
		}
		out = append(out, normalizeAgent(a))
	}
	return out, nil
}

// LookupBySerial resolves a Wazuh agent by board serial from syscollector hardware.
func (c *Client) LookupBySerial(ctx context.Context, serial string) (edrpkg.ManagedEndpoint, error) {
	serial = strings.TrimSpace(serial)
	if serial == "" {
		return edrpkg.ManagedEndpoint{}, edrpkg.ErrDeviceNotFoundInEDR
	}
	ep, _, err := c.lookupForSerial(ctx, serial)
	return ep, err
}

// LookupForHost resolves a Wazuh agent by serial_number only.
func (c *Client) LookupForHost(ctx context.Context, h schema.Host) (edrpkg.ManagedEndpoint, string, error) {
	serial := strings.TrimSpace(h.SerialNumber)
	if serial == "" {
		return edrpkg.ManagedEndpoint{}, "", edrpkg.ErrDeviceNotFoundInEDR
	}
	return c.lookupForSerial(ctx, serial)
}

func (c *Client) lookupForSerial(ctx context.Context, serial string) (edrpkg.ManagedEndpoint, string, error) {
	hardware, _, err := c.listHardwarePage(ctx, 0, 1, serial)
	if err != nil {
		return edrpkg.ManagedEndpoint{}, "", err
	}
	if len(hardware) == 0 {
		return edrpkg.ManagedEndpoint{}, "", edrpkg.ErrDeviceNotFoundInEDR
	}
	agentID := strings.TrimSpace(hardware[0].AgentID)
	if agentID == "" {
		return edrpkg.ManagedEndpoint{}, "", edrpkg.ErrDeviceNotFoundInEDR
	}
	agents, err := c.listAgentsByID(ctx, agentID)
	if err != nil {
		return edrpkg.ManagedEndpoint{}, "", err
	}
	if len(agents) == 0 {
		return edrpkg.ManagedEndpoint{}, "", edrpkg.ErrDeviceNotFoundInEDR
	}
	agents[0].SerialNumber = serial
	return normalizeAgent(agents[0]), schema.EDRMatchSerialNumber, nil
}

func (c *Client) listAllAgents(ctx context.Context) ([]wazuhAgent, error) {
	var out []wazuhAgent
	for offset := 0; ; {
		page, total, err := c.listAgentsPage(ctx, offset, defaultLimit)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		offset += len(page)
		if len(page) == 0 || offset >= total {
			break
		}
	}
	return out, nil
}

func (c *Client) listAgentsByID(ctx context.Context, agentID string) ([]wazuhAgent, error) {
	tok, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	u := c.baseURL + agentsPath + "?agents_list=" + url.QueryEscape(agentID) + "&limit=1"
	var data listResponse[wazuhAgent]
	if err := c.apiGet(ctx, tok, u, &data); err != nil {
		return nil, err
	}
	return data.AffectedItems, nil
}

func (c *Client) listAgentsPage(ctx context.Context, offset, limit int) ([]wazuhAgent, int, error) {
	tok, err := c.accessToken(ctx)
	if err != nil {
		return nil, 0, err
	}
	u := fmt.Sprintf("%s%s?offset=%d&limit=%d", c.baseURL, agentsPath, offset, limit)
	var data listResponse[wazuhAgent]
	if err := c.apiGet(ctx, tok, u, &data); err != nil {
		return nil, 0, err
	}
	return data.AffectedItems, data.TotalAffectedItems, nil
}

func (c *Client) listHardwareSerials(ctx context.Context) (map[string]string, error) {
	out := make(map[string]string)
	for offset := 0; ; {
		page, total, err := c.listHardwarePage(ctx, offset, defaultLimit, "")
		if err != nil {
			return nil, err
		}
		for _, item := range page {
			agentID := strings.TrimSpace(item.AgentID)
			serial := strings.TrimSpace(item.BoardSerial)
			if agentID != "" && serial != "" {
				out[agentID] = serial
			}
		}
		offset += len(page)
		if len(page) == 0 || offset >= total {
			break
		}
	}
	return out, nil
}

func (c *Client) listHardwarePage(ctx context.Context, offset, limit int, boardSerial string) ([]wazuhHardware, int, error) {
	tok, err := c.accessToken(ctx)
	if err != nil {
		return nil, 0, err
	}
	u := fmt.Sprintf("%s%s?offset=%d&limit=%d", c.baseURL, hardwarePath, offset, limit)
	if boardSerial != "" {
		u += "&board_serial=" + url.QueryEscape(boardSerial)
	}
	var data listResponse[wazuhHardware]
	if err := c.apiGet(ctx, tok, u, &data); err != nil {
		return nil, 0, err
	}
	return data.AffectedItems, data.TotalAffectedItems, nil
}

func (c *Client) apiGet(ctx context.Context, tok, u string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return wrapWazuhConnError("api request", err)
	}
	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("wazuh api: http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if readErr != nil {
		return readErr
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return err
	}
	if envelope.Error != 0 {
		return fmt.Errorf("wazuh api error %d", envelope.Error)
	}
	if err := json.Unmarshal(envelope.Data, dest); err != nil {
		return err
	}
	return nil
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.token != "" && time.Until(c.tokenExp) > time.Minute {
		return c.token, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+authPath, nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", wrapWazuhConnError("authenticate", err)
	}
	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("wazuh auth: http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if readErr != nil {
		return "", readErr
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return "", err
	}
	if envelope.Error != 0 {
		return "", fmt.Errorf("wazuh auth error %d", envelope.Error)
	}
	var authData struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(envelope.Data, &authData); err != nil {
		return "", err
	}
	if authData.Token == "" {
		return "", fmt.Errorf("wazuh auth: empty token")
	}
	c.token = authData.Token
	c.tokenExp = time.Now().Add(14 * time.Minute)
	return c.token, nil
}

func normalizeAgent(a wazuhAgent) edrpkg.ManagedEndpoint {
	last := time.Time{}
	if a.LastKeepAlive != "" {
		if ts, err := time.Parse(time.RFC3339, a.LastKeepAlive); err == nil {
			last = ts
		}
	}
	installed := strings.TrimSpace(a.ID) != "" && a.ID != "000"
	healthy := installed && edrpkg.WazuhHealthyFromStatus(a.Status)
	signals := edrpkg.VendorSignals{
		AgentInstalled:  installed,
		AgentHealthy:    healthy,
		VendorRiskLevel: edrpkg.WazuhRiskFromStatus(a.Status),
	}
	raw, _ := json.Marshal(a)
	return edrpkg.ManagedEndpoint{
		ProviderDeviceID: a.ID,
		SerialNumber:     a.SerialNumber,
		Hostname:         a.Name,
		AgentInstalled:   installed,
		AgentHealthy:     healthy,
		RiskLevel:        edrpkg.ComputeRiskLevel(signals),
		LastSeen:         last,
		RawVendorData:    raw,
	}
}

type apiEnvelope struct {
	Data  json.RawMessage `json:"data"`
	Error int             `json:"error"`
}

type listResponse[T any] struct {
	AffectedItems      []T `json:"affected_items"`
	TotalAffectedItems int `json:"total_affected_items"`
}

type wazuhAgent struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Status        string `json:"status"`
	IP            string `json:"ip"`
	Version       string `json:"version"`
	LastKeepAlive string `json:"lastKeepAlive"`
	SerialNumber  string `json:"-"`
}

type wazuhHardware struct {
	AgentID     string `json:"agent_id"`
	BoardSerial string `json:"board_serial"`
}

func wrapWazuhConnError(op string, err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "x509:") || strings.Contains(msg, "tls:") {
		return fmt.Errorf(
			"%s: TLS certificate verification failed; enable skip TLS verification (insecure_skip_verify) for self-signed manager certificates: %w",
			op, err,
		)
	}
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no such host") || strings.Contains(msg, "i/o timeout") {
		return fmt.Errorf("%s: cannot reach Wazuh manager: %w", op, err)
	}
	return err
}
