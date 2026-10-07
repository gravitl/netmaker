package mdm

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gravitl/netmaker/models"
)

// IntuneProvider implements the models.MDMProvider interface for Microsoft Intune.
type IntuneProvider struct {
	Config      models.MDMConfig
	AccessToken string
	TokenExpiry time.Time
}

// Initialize validates the config and performs an initial OAuth client credentials flow.
func (p *IntuneProvider) Initialize(config models.MDMConfig) error {
	p.Config = config
	if p.Config.TenantID == "" || p.Config.ClientID == "" || p.Config.ClientSecret == "" {
		return errors.New("missing required credentials for Intune provider (tenant_id, client_id, client_secret)")
	}
	
	// Initial token fetch to verify credentials are valid at startup
	return p.refreshToken()
}

// refreshToken fetches a new Azure AD bearer token if the current one is missing or expired.
func (p *IntuneProvider) refreshToken() error {
	// Add a 1-minute buffer to expiry to avoid race conditions
	if time.Now().Add(time.Minute).Before(p.TokenExpiry) && p.AccessToken != "" {
		return nil // Token is still valid
	}

	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", p.Config.TenantID)

	data := url.Values{}
	data.Set("client_id", p.Config.ClientID)
	data.Set("client_secret", p.Config.ClientSecret)
	data.Set("scope", "https://graph.microsoft.com/.default")
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch Intune access token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Intune token API returned status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"` // Usually 3599 seconds
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode Intune token response: %w", err)
	}

	p.AccessToken = tokenResp.AccessToken
	p.TokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return nil
}

// CheckCompliance queries the Microsoft Graph API for the host's Intune status.
func (p *IntuneProvider) CheckCompliance(host models.Host) (models.DevicePosture, error) {
	if err := p.refreshToken(); err != nil {
		return models.DevicePosture{}, err
	}

	// We search Microsoft Graph using the OData $filter query parameter.
	// Intune tracks the Entra (Azure AD) device ID as 'azureADDeviceId' and 
	// the hardware serial number as 'serialNumber'.
	var filter string
	if host.EntraDeviceID != "" {
		filter = fmt.Sprintf("azureADDeviceId eq '%s'", host.EntraDeviceID)
	} else if host.SerialNumber != "" {
		filter = fmt.Sprintf("serialNumber eq '%s'", host.SerialNumber)
	} else {
		return models.DevicePosture{}, errors.New("host is missing both EntraDeviceID and SerialNumber identifiers")
	}

	searchURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/deviceManagement/managedDevices?$filter=%s", url.QueryEscape(filter))

	req, err := http.NewRequest(http.MethodGet, searchURL, nil)
	if err != nil {
		return models.DevicePosture{}, err
	}
	req.Header.Add("Authorization", "Bearer "+p.AccessToken)
	// Suggest JSON responses
	req.Header.Add("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return models.DevicePosture{}, fmt.Errorf("failed to query Microsoft Graph API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return models.DevicePosture{}, fmt.Errorf("Microsoft Graph API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Value []struct {
			ComplianceState string `json:"complianceState"`
			DeviceName      string `json:"deviceName"`
		} `json:"value"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return models.DevicePosture{}, fmt.Errorf("failed to decode Graph API response: %w", err)
	}

	// If no devices matched the filter, it's not enrolled/managed
	if len(result.Value) == 0 {
		return models.DevicePosture{
			IsCompliant:   false,
			LastCheckTime: time.Now(),
			Reason:        "Device not found in Intune",
		}, nil
	}

	// Take the first matching device record
	device := result.Value[0]

	// Intune complianceState string values:
	// "compliant", "noncompliant", "unknown", "conflict", "error", "inGracePeriod", "configManager"
	isCompliant := device.ComplianceState == "compliant" || device.ComplianceState == "inGracePeriod"

	return models.DevicePosture{
		IsCompliant:   isCompliant,
		LastCheckTime: time.Now(),
		Reason:        fmt.Sprintf("Intune compliance state: %s", device.ComplianceState),
	}, nil
}
