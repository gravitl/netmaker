// Package jamf implements an MDM provider backed by Jamf Pro. Self-registers
// with pro/mdm in init(). v1 only reports Enrolled; Compliant is always false.
package jamf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/pro/mdm"
)

const (
	providerName    = "jamf"
	providerDisplay = "Jamf Pro"

	tokenPath      = "/api/oauth/token"
	computersPath  = "/api/v1/computers-inventory"
	computerSects  = "GENERAL,HARDWARE,USER_AND_LOCATION"
	mobileDevPath  = "/api/v2/mobile-devices/detail"
	defaultPageSz  = 200
)

func init() {
	mdm.Register(providerName, providerDisplay, New)
	mdm.RegisterCapabilities(providerName, mdm.Capabilities{ReportsCompliant: false})
}

// New builds a Jamf Pro provider from ServerSettings. Returns an error if
// required credentials are missing.
func New(s models.ServerSettings) (mdm.Provider, error) {
	if s.MDMJamfBaseURL == "" || s.MDMJamfClientID == "" || s.MDMJamfClientSecret == "" {
		return nil, errors.New("jamf credentials not configured")
	}
	return &Client{
		baseURL:      strings.TrimRight(s.MDMJamfBaseURL, "/"),
		clientID:     s.MDMJamfClientID,
		clientSecret: s.MDMJamfClientSecret,
		http:         &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Client implements mdm.Provider against Jamf Pro.
type Client struct {
	baseURL      string
	clientID     string
	clientSecret string
	http         *http.Client

	tokenMu  sync.Mutex
	token    string
	tokenExp time.Time
}

// Name implements mdm.Provider.
func (c *Client) Name() string { return providerName }

// Capabilities implements mdm.Provider.
func (c *Client) Capabilities() mdm.Capabilities {
	return mdm.Capabilities{ReportsCompliant: false}
}

// Verify implements mdm.Provider.
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

// ListManagedDevices implements mdm.Provider. Aggregates computers + mobiles.
func (c *Client) ListManagedDevices(ctx context.Context) ([]mdm.ManagedDevice, error) {
	tok, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	out := []mdm.ManagedDevice{}
	macs, err := c.listComputers(ctx, tok)
	if err != nil {
		return nil, err
	}
	out = append(out, macs...)
	mobs, err := c.listMobileDevices(ctx, tok)
	if err != nil {
		// macOS results may be useful even if mobile fails; log via error wrap.
		return out, fmt.Errorf("jamf list mobile-devices: %w", err)
	}
	out = append(out, mobs...)
	return out, nil
}

func (c *Client) listComputers(ctx context.Context, tok string) ([]mdm.ManagedDevice, error) {
	var out []mdm.ManagedDevice
	for page := 0; ; page++ {
		u := fmt.Sprintf("%s%s?page=%d&page-size=%d&section=%s",
			c.baseURL, computersPath, page, defaultPageSz, computerSects)
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
		var body computerInventoryPage
		err = json.NewDecoder(resp.Body).Decode(&body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}
		for _, r := range body.Results {
			out = append(out, normalizeComputer(r))
		}
		if len(body.Results) < defaultPageSz {
			break
		}
	}
	return out, nil
}

func (c *Client) listMobileDevices(ctx context.Context, tok string) ([]mdm.ManagedDevice, error) {
	var out []mdm.ManagedDevice
	for page := 0; ; page++ {
		u := fmt.Sprintf("%s%s?page=%d&page-size=%d",
			c.baseURL, mobileDevPath, page, defaultPageSz)
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
		var body mobileDevicesPage
		err = json.NewDecoder(resp.Body).Decode(&body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}
		for _, r := range body.Results {
			out = append(out, normalizeMobile(r))
		}
		if len(body.Results) < defaultPageSz {
			break
		}
	}
	return out, nil
}

// accessToken returns a cached OAuth bearer, refreshing within 60s of expiry.
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

func normalizeComputer(r computerInventory) mdm.ManagedDevice {
	last, _ := time.Parse(time.RFC3339, r.General.LastContactTime)
	return mdm.ManagedDevice{
		ProviderDeviceID:  r.ID,
		SerialNumber:      r.Hardware.SerialNumber,
		HardwareUUID:      r.General.UDID,
		DeviceName:        r.General.Name,
		UserPrincipalName: r.UserAndLocation.EmailAddress,
		Enrolled:          true,
		Compliant:         false,
		LastSeenAt:        last,
	}
}

func normalizeMobile(r mobileDevice) mdm.ManagedDevice {
	last, _ := time.Parse(time.RFC3339, r.LastInventoryUpdateDate)
	return mdm.ManagedDevice{
		ProviderDeviceID:  r.ID,
		SerialNumber:      r.SerialNumber,
		HardwareUUID:      r.UDID,
		DeviceName:        r.Name,
		UserPrincipalName: r.Location.EmailAddress,
		Enrolled:          true,
		Compliant:         false,
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
