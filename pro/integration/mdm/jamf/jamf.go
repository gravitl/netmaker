// Package jamf implements an MDM provider backed by Jamf Pro. Self-registers
// with pro/integration/mdm in init().
package jamf

import (
	"context"
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
	providerName    = mdmpkg.ProviderJamf
	providerDisplay = "Jamf Pro"

	tokenPath              = "/api/oauth/token"
	computersPath          = "/api/v1/computers-inventory"
	computerCompliancePath = "/api/v1/conditional-access/device-compliance-information/computer/"
	mobileDevPath          = "/api/v2/mobile-devices/detail"
	mobileCompliancePath   = "/api/v1/conditional-access/device-compliance-information/mobile/"
	computerSects          = "GENERAL,HARDWARE,USER_AND_LOCATION"
	defaultPageSz          = 200
	maxComplianceFetches   = 8
)

func init() {
	mdmpkg.Register(providerName, providerDisplay, New)
	mdmpkg.RegisterCapabilities(providerName, mdmpkg.Capabilities{ReportsCompliant: true})
}

// New builds a Jamf Pro provider from integration config JSON.
func New(configJSON json.RawMessage) (mdmpkg.Provider, error) {
	var cfg mdmpkg.JamfConfig
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, fmt.Errorf("invalid jamf config: %w", err)
	}
	if err := mdmpkg.ValidateConfig(providerName, configJSON); err != nil {
		return nil, err
	}
	return &Client{
		baseURL:           strings.TrimRight(cfg.BaseURL, "/"),
		clientID:          cfg.ClientID,
		clientSecret:      cfg.ClientSecret,
		complianceVendors: complianceVendorSet(cfg.ComplianceVendors),
		http:              &http.Client{Timeout: 60 * time.Second},
	}, nil
}

// Client implements mdmpkg.Provider against Jamf Pro.
type Client struct {
	baseURL           string
	clientID          string
	clientSecret      string
	complianceVendors map[string]struct{}
	http              *http.Client

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
	u := fmt.Sprintf("%s%s?page-size=1&page=0&section=GENERAL", c.baseURL, computersPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("jamf verify failed: http %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) ListManagedDevices(ctx context.Context) ([]mdmpkg.ManagedDevice, error) {
	tok, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	out := []mdmpkg.ManagedDevice{}
	computers, err := c.listComputers(ctx, tok)
	if err != nil {
		return nil, err
	}
	compliance, err := c.fetchComputerCompliance(ctx, tok, computers)
	if err != nil {
		return nil, err
	}
	for _, r := range computers {
		compliant := compliance[r.ID]
		out = append(out, normalizeComputer(r, compliant))
	}
	mobiles, err := c.listMobileDevices(ctx, tok)
	if err != nil {
		return out, fmt.Errorf("jamf list mobile-devices: %w", err)
	}
	mobileCompliance, err := c.fetchMobileCompliance(ctx, tok, mobiles)
	if err != nil {
		return out, fmt.Errorf("jamf mobile compliance: %w", err)
	}
	for _, r := range mobiles {
		compliant := mobileCompliance[r.ID]
		out = append(out, normalizeMobile(r, compliant))
	}
	return out, nil
}

func (c *Client) listComputers(ctx context.Context, tok string) ([]computerInventory, error) {
	var out []computerInventory
	for pageNum := 0; ; pageNum++ {
		u := fmt.Sprintf("%s%s?page=%d&page-size=%d&section=%s",
			c.baseURL, computersPath, pageNum, defaultPageSz, computerSects)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Accept", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("jamf computers-inventory: http %d", resp.StatusCode)
		}
		if readErr != nil {
			return nil, readErr
		}
		var pageBody computerInventoryPage
		if err := json.Unmarshal(body, &pageBody); err != nil {
			return nil, err
		}
		out = append(out, pageBody.Results...)
		if len(pageBody.Results) < defaultPageSz {
			break
		}
	}
	return out, nil
}

func (c *Client) listMobileDevices(ctx context.Context, tok string) ([]mobileDevice, error) {
	var out []mobileDevice
	for pageNum := 0; ; pageNum++ {
		u := fmt.Sprintf("%s%s?page=%d&page-size=%d",
			c.baseURL, mobileDevPath, pageNum, defaultPageSz)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Accept", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("jamf mobile-devices: http %d", resp.StatusCode)
		}
		if readErr != nil {
			return nil, readErr
		}
		var pageBody mobileDevicesPage
		if err := json.Unmarshal(body, &pageBody); err != nil {
			return nil, err
		}
		out = append(out, pageBody.Results...)
		if len(pageBody.Results) < defaultPageSz {
			break
		}
	}
	return out, nil
}

func (c *Client) fetchComputerCompliance(ctx context.Context, tok string, computers []computerInventory) (map[string]bool, error) {
	return c.fetchCompliance(ctx, tok, len(computers), func(i int) (string, error) {
		return computers[i].ID, nil
	}, computerCompliancePath)
}

func (c *Client) fetchMobileCompliance(ctx context.Context, tok string, mobiles []mobileDevice) (map[string]bool, error) {
	return c.fetchCompliance(ctx, tok, len(mobiles), func(i int) (string, error) {
		return mobiles[i].ID, nil
	}, mobileCompliancePath)
}

func (c *Client) fetchCompliance(
	ctx context.Context,
	tok string,
	n int,
	deviceID func(int) (string, error),
	pathPrefix string,
) (map[string]bool, error) {
	out := make(map[string]bool, n)
	if n == 0 {
		return out, nil
	}
	sem := make(chan struct{}, maxComplianceFetches)
	type result struct {
		id        string
		compliant bool
		err       error
	}
	ch := make(chan result, n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			id, err := deviceID(i)
			if err != nil {
				ch <- result{err: err}
				return
			}
			records, err := c.getDeviceCompliance(ctx, tok, pathPrefix, id)
			if err != nil {
				ch <- result{id: id, err: err}
				return
			}
			ch <- result{
				id:        id,
				compliant: jamfDeviceTrustCompliant(records, c.complianceVendors),
			}
		}()
	}
	var firstErr error
	success := 0
	for range n {
		r := <-ch
		if r.err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("device %s compliance: %w", r.id, r.err)
			}
			if r.id != "" {
				out[r.id] = false
			}
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

func (c *Client) getDeviceCompliance(ctx context.Context, tok, pathPrefix, deviceID string) ([]deviceComplianceInfo, error) {
	u := c.baseURL + pathPrefix + url.PathEscape(deviceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		// Conditional Access compliance not configured for this device.
		return nil, nil
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var records []deviceComplianceInfo
	if err := json.Unmarshal(respBody, &records); err != nil {
		return nil, err
	}
	return records, nil
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.token != "" && time.Until(c.tokenExp) > time.Minute {
		return c.token, nil
	}
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		c.baseURL+tokenPath,
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", errors.New("jamf token: " + err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("jamf token: http %d", resp.StatusCode)
	}
	var body tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	if body.AccessToken == "" {
		return "", errors.New("jamf token: empty response")
	}
	c.token = body.AccessToken
	if body.ExpiresIn > 0 {
		c.tokenExp = time.Now().Add(time.Duration(body.ExpiresIn) * time.Second)
	} else {
		c.tokenExp = time.Now().Add(20 * time.Minute)
	}
	return c.token, nil
}

func normalizeComputer(r computerInventory, compliant bool) mdmpkg.ManagedDevice {
	last, _ := time.Parse(time.RFC3339, r.General.LastContactTime)
	return mdmpkg.ManagedDevice{
		ProviderDeviceID:  r.ID,
		SerialNumber:      r.Hardware.SerialNumber,
		HardwareUUID:      r.General.UDID,
		DeviceName:        r.General.Name,
		UserPrincipalName: r.UserAndLocation.EmailAddress,
		Enrolled:          true,
		Compliant:         compliant,
		LastSeenAt:        last,
	}
}

func normalizeMobile(r mobileDevice, compliant bool) mdmpkg.ManagedDevice {
	last, _ := time.Parse(time.RFC3339, r.LastInventoryUpdateDate)
	return mdmpkg.ManagedDevice{
		ProviderDeviceID:  r.ID,
		SerialNumber:      r.SerialNumber,
		HardwareUUID:      r.UDID,
		DeviceName:        r.Name,
		UserPrincipalName: r.Location.EmailAddress,
		Enrolled:          true,
		Compliant:         compliant,
		LastSeenAt:        last,
	}
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type computerInventoryPage struct {
	TotalCount int                 `json:"totalCount"`
	Results    []computerInventory `json:"results"`
}

type computerInventory struct {
	ID              string                  `json:"id"`
	General         computerGeneral         `json:"general"`
	Hardware        computerHardware        `json:"hardware"`
	UserAndLocation computerUserAndLocation `json:"userAndLocation"`
}

type computerGeneral struct {
	Name            string `json:"name"`
	UDID            string `json:"udid"`
	LastContactTime string `json:"lastContactTime"`
}

type computerHardware struct {
	SerialNumber string `json:"serialNumber"`
}

type computerUserAndLocation struct {
	EmailAddress string `json:"email"`
}

type mobileDevicesPage struct {
	TotalCount int            `json:"totalCount"`
	Results    []mobileDevice `json:"results"`
}

type mobileDevice struct {
	ID                      string         `json:"id"`
	Name                    string         `json:"name"`
	UDID                    string         `json:"udid"`
	SerialNumber            string         `json:"serialNumber"`
	LastInventoryUpdateDate string         `json:"lastInventoryUpdateDate"`
	Location                mobileLocation `json:"location"`
}

type mobileLocation struct {
	EmailAddress string `json:"emailAddress"`
}
