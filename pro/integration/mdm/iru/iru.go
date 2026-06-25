// Package iru implements an MDM provider backed by Iru Endpoint Management
// (formerly Kandji). Self-registers with pro/integration/mdm in init().
package iru

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	mdmpkg "github.com/gravitl/netmaker/pro/integration/mdm"
)

const (
	providerName         = mdmpkg.ProviderIru
	providerDisplay      = "Iru"
	devicesListPath      = "/api/v1/devices"
	deviceStatusPathFmt  = "/api/v1/devices/%s/status"
	defaultPageSize      = 300
	maxStatusFetches     = 8
)

func init() {
	mdmpkg.Register(providerName, providerDisplay, New)
	mdmpkg.RegisterCapabilities(providerName, mdmpkg.Capabilities{ReportsCompliant: true})
}

// New builds an Iru provider from integration config JSON.
func New(configJSON json.RawMessage) (mdmpkg.Provider, error) {
	var cfg mdmpkg.IruConfig
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, fmt.Errorf("invalid iru config: %w", err)
	}
	if err := mdmpkg.ValidateConfig(providerName, configJSON); err != nil {
		return nil, err
	}
	return &Client{
		baseURL:              strings.TrimRight(strings.TrimSpace(cfg.APIURL), "/"),
		apiToken:             cfg.APIToken,
		complianceLibraryIDs: libraryItemIDSet(cfg.ComplianceLibraryItemIDs),
		http:                 &http.Client{Timeout: 60 * time.Second},
	}, nil
}

// Client implements mdmpkg.Provider against the Iru Endpoint Management API.
type Client struct {
	baseURL              string
	apiToken             string
	complianceLibraryIDs map[string]struct{}
	http                 *http.Client
}

func (c *Client) Name() string { return providerName }

func (c *Client) Capabilities() mdmpkg.Capabilities {
	return mdmpkg.Capabilities{ReportsCompliant: true}
}

func (c *Client) Verify(ctx context.Context) error {
	devices, err := c.listDevices(ctx, 0, 1)
	if err != nil {
		return fmt.Errorf("iru verify failed: %w", err)
	}
	if len(devices) == 0 {
		return nil
	}
	if _, err := c.getDeviceStatus(ctx, devices[0].DeviceID); err != nil {
		return fmt.Errorf("iru verify failed: %w", err)
	}
	return nil
}

func (c *Client) ListManagedDevices(ctx context.Context) ([]mdmpkg.ManagedDevice, error) {
	var devices []iruDevice
	for offset := 0; ; offset += defaultPageSize {
		page, err := c.listDevices(ctx, offset, defaultPageSize)
		if err != nil {
			return nil, err
		}
		devices = append(devices, page...)
		if len(page) < defaultPageSize {
			break
		}
	}
	complianceByID, err := c.fetchDeviceCompliance(ctx, devices)
	if err != nil {
		return nil, err
	}
	out := make([]mdmpkg.ManagedDevice, 0, len(devices))
	for _, d := range devices {
		compliant, ok := complianceByID[d.DeviceID]
		if !ok {
			compliant = false
		}
		out = append(out, normalize(d, compliant))
	}
	return out, nil
}

func (c *Client) fetchDeviceCompliance(ctx context.Context, devices []iruDevice) (map[string]bool, error) {
	out := make(map[string]bool, len(devices))
	if len(devices) == 0 {
		return out, nil
	}
	sem := make(chan struct{}, maxStatusFetches)
	type result struct {
		id        string
		compliant bool
		err       error
	}
	ch := make(chan result, len(devices))
	for _, d := range devices {
		d := d
		go func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			status, err := c.getDeviceStatus(ctx, d.DeviceID)
			if err != nil {
				ch <- result{id: d.DeviceID, err: err}
				return
			}
			ch <- result{
				id:        d.DeviceID,
				compliant: deviceCompliant(status, c.complianceLibraryIDs),
			}
		}()
	}
	var firstErr error
	success := 0
	for range devices {
		r := <-ch
		if r.err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("device %s status: %w", r.id, r.err)
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

func (c *Client) listDevices(ctx context.Context, offset, limit int) ([]iruDevice, error) {
	u, err := url.Parse(c.baseURL + devicesListPath)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("offset", fmt.Sprintf("%d", offset))
	q.Set("limit", fmt.Sprintf("%d", limit))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("iru list devices: http %d", resp.StatusCode)
	}

	var page devicesListResponse
	if err := json.Unmarshal(body, &page); err == nil {
		return page.Devices, nil
	}
	var devices []iruDevice
	if err := json.Unmarshal(body, &devices); err != nil {
		return nil, err
	}
	return devices, nil
}

func (c *Client) getDeviceStatus(ctx context.Context, deviceID string) (iruDeviceStatus, error) {
	u := c.baseURL + fmt.Sprintf(deviceStatusPathFmt, url.PathEscape(deviceID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return iruDeviceStatus{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return iruDeviceStatus{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return iruDeviceStatus{}, fmt.Errorf("http %d", resp.StatusCode)
	}
	var status iruDeviceStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return iruDeviceStatus{}, err
	}
	return status, nil
}

func normalize(d iruDevice, compliant bool) mdmpkg.ManagedDevice {
	last, _ := time.Parse(time.RFC3339, d.LastCheckIn)
	email := d.UserEmail
	if email == "" {
		email = d.User.Email()
	}
	return mdmpkg.ManagedDevice{
		ProviderDeviceID:  d.DeviceID,
		SerialNumber:      d.SerialNumber,
		DeviceName:        d.DeviceName,
		UserPrincipalName: email,
		Enrolled:          true,
		Compliant:         compliant,
		LastSeenAt:        last,
	}
}

type devicesListResponse struct {
	Devices []iruDevice `json:"devices"`
}

type iruDevice struct {
	DeviceID     string      `json:"device_id"`
	DeviceName   string      `json:"device_name"`
	SerialNumber string      `json:"serial_number"`
	LastCheckIn  string      `json:"last_check_in"`
	UserEmail    string      `json:"user_email"`
	User         iruUserField `json:"user"`
}

// iruUserField accepts Iru/Kandji "user" as either a string (email) or object.
type iruUserField struct {
	email string
}

func (u iruUserField) Email() string {
	return u.email
}

func (u *iruUserField) UnmarshalJSON(data []byte) error {
	u.email = ""
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		u.email = strings.TrimSpace(s)
		return nil
	}
	var obj struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	u.email = strings.TrimSpace(obj.Email)
	return nil
}
