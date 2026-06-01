// Package intune implements an MDM provider backed by Microsoft Intune via
// Microsoft Graph. Self-registers with pro/mdm in init().
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

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/pro/mdm"
)

const (
	providerName    = "intune"
	providerDisplay = "Microsoft Intune"

	tokenURLFmt  = "https://login.microsoftonline.com/%s/oauth2/v2.0/token"
	tokenScope   = "https://graph.microsoft.com/.default"
	devicesURL   = "https://graph.microsoft.com/v1.0/deviceManagement/managedDevices"
	deviceSelect = "id,azureADDeviceId,serialNumber,hardwareInformation,deviceName,userPrincipalName,managementState,complianceState,lastSyncDateTime"
)

func init() {
	mdm.Register(providerName, providerDisplay, New)
	mdm.RegisterCapabilities(providerName, mdm.Capabilities{ReportsCompliant: true})
}

// New builds an Intune provider from ServerSettings. Returns an error if
// required credentials are missing.
func New(s models.ServerSettings) (mdm.Provider, error) {
	if s.MDMIntuneTenantID == "" || s.MDMClientID == "" || s.MDMClientSecret == "" {
		return nil, errors.New("intune credentials not configured")
	}
	return &Client{
		tenantID:     s.MDMIntuneTenantID,
		clientID:     s.MDMClientID,
		clientSecret: s.MDMClientSecret,
		http:         &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Client implements mdm.Provider against Microsoft Graph.
type Client struct {
	tenantID     string
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
	return mdm.Capabilities{ReportsCompliant: true}
}

// Verify implements mdm.Provider.
func (c *Client) Verify(ctx context.Context) error {
	tok, err := c.accessToken(ctx)
	if err != nil {
		return err
	}
	u := devicesURL + "?$top=1&$select=id"
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

// ListManagedDevices implements mdm.Provider. Iterates @odata.nextLink.
func (c *Client) ListManagedDevices(ctx context.Context) ([]mdm.ManagedDevice, error) {
	tok, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	nextURL := devicesURL + "?$select=" + url.QueryEscape(deviceSelect)
	var out []mdm.ManagedDevice
	for nextURL != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, nextURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Accept", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		var page managedDevicesPage
		err = json.NewDecoder(resp.Body).Decode(&page)
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if page.Error.Code != "" {
			return nil, fmt.Errorf("intune list devices: %s", page.Error.Message)
		}
		for _, d := range page.Value {
			out = append(out, normalize(d))
		}
		nextURL = page.NextLink
	}
	return out, nil
}

// accessToken returns a cached token, refreshing when within 60s of expiry.
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

func normalize(d managedDevice) mdm.ManagedDevice {
	last, _ := time.Parse(time.RFC3339, d.LastSyncDateTime)
	return mdm.ManagedDevice{
		ProviderDeviceID:  d.ID,
		AzureADDeviceID:   d.AzureADDeviceID,
		SerialNumber:      d.SerialNumber,
		HardwareUUID:      d.HardwareInformation.SerialNumber, // best-effort: SMBIOS UUID is not exposed on managedDevices; fall back to serial
		DeviceName:        d.DeviceName,
		UserPrincipalName: d.UserPrincipalName,
		Enrolled:          d.ManagementState != "" && !strings.EqualFold(d.ManagementState, "discovered"),
		Compliant:         strings.EqualFold(d.ComplianceState, "compliant"),
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
	ID                  string              `json:"id"`
	AzureADDeviceID     string              `json:"azureADDeviceId"`
	SerialNumber        string              `json:"serialNumber"`
	DeviceName          string              `json:"deviceName"`
	UserPrincipalName   string              `json:"userPrincipalName"`
	ManagementState     string              `json:"managementState"`
	ComplianceState     string              `json:"complianceState"`
	LastSyncDateTime    string              `json:"lastSyncDateTime"`
	HardwareInformation hardwareInformation `json:"hardwareInformation"`
}

type hardwareInformation struct {
	SerialNumber string `json:"serialNumber"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
