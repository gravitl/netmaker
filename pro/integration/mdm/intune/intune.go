// Package intune implements an MDM provider backed by Microsoft Intune via
// Microsoft Graph. Self-registers with pro/integration/mdm in init().
package intune

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

	mdmpkg "github.com/gravitl/netmaker/pro/integration/mdm"
)

const (
	providerName    = mdmpkg.ProviderIntune
	providerDisplay = "Microsoft Intune"

	tokenURLFmt     = "https://login.microsoftonline.com/%s/oauth2/v2.0/token"
	tokenScope      = "https://graph.microsoft.com/.default"
	entraDevicesURL = "https://graph.microsoft.com/v1.0/devices"
	devicesURL      = "https://graph.microsoft.com/v1.0/deviceManagement/managedDevices"
	deviceSelect    = "id,azureADDeviceId,serialNumber,deviceName,userPrincipalName,managementState,deviceRegistrationState,enrolledDateTime,complianceState,lastSyncDateTime"
)

func init() {
	mdmpkg.Register(providerName, providerDisplay, New)
	mdmpkg.RegisterCapabilities(providerName, mdmpkg.Capabilities{ReportsCompliant: true})
}

// New builds an Intune provider from integration config JSON.
func New(configJSON json.RawMessage) (mdmpkg.Provider, error) {
	var cfg mdmpkg.IntuneConfig
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, fmt.Errorf("invalid intune config: %w", err)
	}
	if err := mdmpkg.ValidateConfig(providerName, configJSON); err != nil {
		return nil, err
	}
	return &Client{
		tenantID:     cfg.TenantID,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		http:         &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Client implements mdmpkg.Provider against Microsoft Graph.
type Client struct {
	tenantID     string
	clientID     string
	clientSecret string
	http         *http.Client

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
	u := entraDevicesURL + "?$top=1&$select=" + url.QueryEscape("id")
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
	var body struct {
		Error errorBody `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if resp.StatusCode >= 400 || body.Error.Code != "" {
		if body.Error.Message != "" {
			return fmt.Errorf("intune verify failed: %s", body.Error.Message)
		}
		return fmt.Errorf("intune verify failed: http %d", resp.StatusCode)
	}
	return nil
}

// ListManagedDevices returns Intune managed devices for serial_number matching
// when a host has no entra_device_id. Entra-keyed posture checks use
// LookupByEntraDeviceID instead.
func (c *Client) ListManagedDevices(ctx context.Context) ([]mdmpkg.ManagedDevice, error) {
	tok, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	raw, err := c.listAllManagedDevices(ctx, tok)
	if err != nil {
		return nil, err
	}
	out := make([]mdmpkg.ManagedDevice, 0, len(raw))
	for _, d := range raw {
		out = append(out, normalize(d))
	}
	return out, nil
}

func (c *Client) listAllManagedDevices(ctx context.Context, tok string) ([]managedDevice, error) {
	u := devicesURL + "?$select=" + url.QueryEscape(deviceSelect) + "&$top=999"
	var out []managedDevice
	for u != "" {
		var page managedDevicesPage
		if err := c.graphGet(ctx, tok, u, &page); err != nil {
			return nil, err
		}
		if page.Error.Code != "" {
			return nil, fmt.Errorf("intune list devices: %s", page.Error.Message)
		}
		out = append(out, page.Value...)
		u = page.NextLink
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
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("scope", tokenScope)
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		fmt.Sprintf(tokenURLFmt, c.tenantID),
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", errors.New("intune token: " + err.Error())
	}
	defer resp.Body.Close()
	var body tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	if body.AccessToken == "" {
		if body.Error != "" {
			return "", fmt.Errorf("intune token: %s: %s", body.Error, body.ErrorDescription)
		}
		return "", errors.New("intune token: empty response")
	}
	c.token = body.AccessToken
	if body.ExpiresIn > 0 {
		c.tokenExp = time.Now().Add(time.Duration(body.ExpiresIn) * time.Second)
	} else {
		c.tokenExp = time.Now().Add(50 * time.Minute)
	}
	return c.token, nil
}

// intuneComplianceCompliant is true only when Graph reports complianceState
// as "compliant" (case-insensitive).
func intuneComplianceCompliant(state string) bool {
	return strings.EqualFold(strings.TrimSpace(state), "compliant")
}

// intuneDeviceEnrolled reports whether a managedDevices row represents an
// enrolled Intune device.
func intuneDeviceEnrolled(d managedDevice) bool {
	if d.ManagementState != "" && !strings.EqualFold(d.ManagementState, "discovered") {
		return true
	}
	if strings.EqualFold(d.DeviceRegistrationState, "registered") {
		return true
	}
	if strings.TrimSpace(d.EnrolledDateTime) != "" {
		return true
	}
	return false
}

func normalize(d managedDevice) mdmpkg.ManagedDevice {
	last, _ := time.Parse(time.RFC3339, d.LastSyncDateTime)
	return mdmpkg.ManagedDevice{
		ProviderDeviceID:  d.ID,
		AzureADDeviceID:   d.AzureADDeviceID,
		SerialNumber:      d.SerialNumber,
		DeviceName:        d.DeviceName,
		UserPrincipalName: d.UserPrincipalName,
		Enrolled:          intuneDeviceEnrolled(d),
		Compliant:         intuneComplianceCompliant(d.ComplianceState),
		LastSeenAt:        last,
	}
}

type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type managedDevicesPage struct {
	Value    []managedDevice `json:"value"`
	NextLink string          `json:"@odata.nextLink"`
	Error    errorBody       `json:"error"`
}

type managedDevice struct {
	ID                      string              `json:"id"`
	AzureADDeviceID         string              `json:"azureADDeviceId"`
	SerialNumber            string              `json:"serialNumber"`
	DeviceName              string              `json:"deviceName"`
	UserPrincipalName       string              `json:"userPrincipalName"`
	ManagementState         string              `json:"managementState"`
	DeviceRegistrationState string              `json:"deviceRegistrationState"`
	EnrolledDateTime        string              `json:"enrolledDateTime"`
	ComplianceState         string              `json:"complianceState"`
	LastSyncDateTime        string `json:"lastSyncDateTime"`
}

type entraDevice struct {
	ID              string `json:"id"`
	DeviceID        string `json:"deviceId"`
	DisplayName     string `json:"displayName"`
	OperatingSystem string `json:"operatingSystem"`
	TrustType       string `json:"trustType"`
	IsManaged       bool   `json:"isManaged"`
	IsCompliant     bool   `json:"isCompliant"`
}

type entraDevicesPage struct {
	Value []entraDevice `json:"value"`
	Error errorBody     `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
