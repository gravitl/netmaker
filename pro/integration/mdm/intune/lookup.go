package intune

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	mdmpkg "github.com/gravitl/netmaker/pro/integration/mdm"
)

const (
	entraDeviceSelect         = "id,deviceId,displayName,operatingSystem,trustType,isManaged,isCompliant"
	managedDeviceBackupSelect = "id,deviceName,azureADDeviceId,complianceState,managementState,deviceRegistrationState,enrolledDateTime"
)

// LookupByEntraDeviceID resolves a host using entra_device_id as Graph devices.deviceId.
//  1. GET /v1.0/devices?$filter=deviceId eq '<id>' — isManaged / isCompliant
//  2. If no match, GET /deviceManagement/managedDevices?$filter=azureADDeviceId eq '<id>'
//     — complianceState == "compliant"
func (c *Client) LookupByEntraDeviceID(ctx context.Context, entraDeviceID string) (mdmpkg.ManagedDevice, error) {
	entraDeviceID = normalizeEntraGUID(entraDeviceID)
	if entraDeviceID == "" {
		return mdmpkg.ManagedDevice{}, mdmpkg.ErrDeviceNotRegisteredInEntra
	}

	tok, err := c.accessToken(ctx)
	if err != nil {
		return mdmpkg.ManagedDevice{}, err
	}

	entraDevices, err := c.queryEntraDevicesByDeviceID(ctx, tok, entraDeviceID)
	if err != nil {
		return mdmpkg.ManagedDevice{}, err
	}
	if len(entraDevices) > 0 {
		return managedFromEntraDevice(entraDevices[0], entraDeviceID), nil
	}

	// /devices returned no row — managedDevices is fallback only, never called above.
	return c.lookupManagedDeviceFallback(ctx, tok, entraDeviceID)
}

func (c *Client) lookupManagedDeviceFallback(ctx context.Context, tok, entraDeviceID string) (mdmpkg.ManagedDevice, error) {
	managed, err := c.queryManagedDevicesBackup(ctx, tok, entraDeviceID)
	if err != nil {
		return mdmpkg.ManagedDevice{}, err
	}
	if len(managed) == 0 {
		return mdmpkg.ManagedDevice{}, mdmpkg.ErrDeviceNotRegisteredInEntra
	}
	return managedFromManagedDeviceBackup(managed[0], entraDeviceID), nil
}

func managedFromEntraDevice(e entraDevice, entraDeviceID string) mdmpkg.ManagedDevice {
	deviceID := entraDeviceID
	if e.DeviceID != "" {
		deviceID = normalizeEntraGUID(e.DeviceID)
	}
	return mdmpkg.ManagedDevice{
		ProviderDeviceID: e.ID,
		AzureADDeviceID:  deviceID,
		DeviceName:       e.DisplayName,
		Enrolled:         e.IsManaged,
		Compliant:        e.IsCompliant,
	}
}

func managedFromManagedDeviceBackup(d managedDevice, entraDeviceID string) mdmpkg.ManagedDevice {
	return mdmpkg.ManagedDevice{
		ProviderDeviceID: d.ID,
		AzureADDeviceID:  entraDeviceID,
		DeviceName:       d.DeviceName,
		Enrolled:         intuneDeviceEnrolled(d),
		Compliant:        intuneComplianceCompliant(d.ComplianceState),
	}
}

func (c *Client) queryEntraDevicesByDeviceID(ctx context.Context, tok, deviceID string) ([]entraDevice, error) {
	filter := "deviceId eq '" + odataQuote(deviceID) + "'"
	u := entraDevicesURL + "?$filter=" + url.QueryEscape(filter) +
		"&$select=" + url.QueryEscape(entraDeviceSelect)
	var page entraDevicesPage
	if err := c.graphGet(ctx, tok, u, &page); err != nil {
		return nil, err
	}
	if page.Error.Code != "" {
		return nil, fmt.Errorf("entra list devices: %s", page.Error.Message)
	}
	return page.Value, nil
}

func (c *Client) queryManagedDevicesBackup(ctx context.Context, tok, entraDeviceID string) ([]managedDevice, error) {
	filter := "azureADDeviceId eq '" + odataQuote(entraDeviceID) + "'"
	u := devicesURL + "?$filter=" + url.QueryEscape(filter) +
		"&$select=" + url.QueryEscape(managedDeviceBackupSelect)
	var page managedDevicesPage
	if err := c.graphGet(ctx, tok, u, &page); err != nil {
		return nil, err
	}
	if page.Error.Code != "" {
		return nil, fmt.Errorf("intune list devices: %s", page.Error.Message)
	}
	return page.Value, nil
}

func (c *Client) graphGet(ctx context.Context, tok, u string, dest any) error {
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
		var body struct {
			Error errorBody `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&body)
		if body.Error.Message != "" {
			return fmt.Errorf("graph http %d: %s", resp.StatusCode, body.Error.Message)
		}
		return fmt.Errorf("graph http %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}

func normalizeEntraGUID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.TrimPrefix(id, "{")
	id = strings.TrimSuffix(id, "}")
	return id
}

func odataQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
