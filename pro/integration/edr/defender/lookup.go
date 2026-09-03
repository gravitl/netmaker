package defender

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	edrpkg "github.com/gravitl/netmaker/pro/integration/edr"
	"github.com/gravitl/netmaker/schema"
)

const graphTokenScope = "https://graph.microsoft.com/.default"

var managedDevicesURL = "https://graph.microsoft.com/v1.0/deviceManagement/managedDevices"

const managedDeviceSelect = "azureADDeviceId,serialNumber"

// LookupForHost resolves a Defender machine by entra_device_id, falling back to
// serial_number via Intune managedDevices in Microsoft Graph when needed.
func (c *Client) LookupForHost(ctx context.Context, h schema.Host) (edrpkg.ManagedEndpoint, string, error) {
	if entra := strings.TrimSpace(h.EntraDeviceID); entra != "" {
		ep, err := c.lookupMachineByEntraID(ctx, entra)
		if err != nil && !errors.Is(err, edrpkg.ErrDeviceNotFoundInEDR) {
			return edrpkg.ManagedEndpoint{}, "", err
		}
		if err == nil {
			return ep, schema.EDRMatchEntraDeviceID, nil
		}
	}
	serial := strings.TrimSpace(h.SerialNumber)
	if serial == "" {
		return edrpkg.ManagedEndpoint{}, "", edrpkg.ErrDeviceNotFoundInEDR
	}
	aadID, err := c.lookupEntraDeviceIDBySerial(ctx, serial)
	if err != nil {
		return edrpkg.ManagedEndpoint{}, "", err
	}
	if aadID == "" {
		return edrpkg.ManagedEndpoint{}, "", edrpkg.ErrDeviceNotFoundInEDR
	}
	ep, err := c.lookupMachineByEntraID(ctx, aadID)
	if err != nil {
		return edrpkg.ManagedEndpoint{}, "", err
	}
	return ep, schema.EDRMatchSerialNumber, nil
}

func (c *Client) lookupMachineByEntraID(ctx context.Context, entraDeviceID string) (edrpkg.ManagedEndpoint, error) {
	entraDeviceID = normalizeGUID(entraDeviceID)
	if entraDeviceID == "" {
		return edrpkg.ManagedEndpoint{}, edrpkg.ErrDeviceNotFoundInEDR
	}
	filter := "aadDeviceId eq '" + odataQuote(entraDeviceID) + "'"
	machines, err := c.listMachinesFiltered(ctx, filter)
	if err != nil {
		return edrpkg.ManagedEndpoint{}, err
	}
	if len(machines) == 0 {
		return edrpkg.ManagedEndpoint{}, edrpkg.ErrDeviceNotFoundInEDR
	}
	return normalizeMachine(machines[0]), nil
}

func (c *Client) listMachinesFiltered(ctx context.Context, filter string) ([]securityMachine, error) {
	tok, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	u := machinesURL + "?$filter=" + url.QueryEscape(filter) + "&$top=" + fmt.Sprintf("%d", defaultPageSz)
	var out []securityMachine
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

func (c *Client) lookupEntraDeviceIDBySerial(ctx context.Context, serial string) (string, error) {
	tok, err := c.graphAccessToken(ctx)
	if err != nil {
		return "", err
	}
	filter := "serialNumber eq '" + odataQuote(serial) + "'"
	u := managedDevicesURL + "?$filter=" + url.QueryEscape(filter) +
		"&$select=" + url.QueryEscape(managedDeviceSelect) + "&$top=1"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("defender graph serial lookup: http %d: %s", resp.StatusCode, defenderAPIError(body))
	}
	var page struct {
		Value []struct {
			AzureADDeviceID string `json:"azureADDeviceId"`
		} `json:"value"`
		Error errorBody `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return "", err
	}
	if page.Error.Code != "" {
		return "", fmt.Errorf("defender graph serial lookup: %s", page.Error.Message)
	}
	if len(page.Value) == 0 {
		return "", nil
	}
	return normalizeGUID(page.Value[0].AzureADDeviceID), nil
}

func (c *Client) graphAccessToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.graphToken != "" && time.Until(c.graphTokenExp) > time.Minute {
		return c.graphToken, nil
	}
	form := url.Values{}
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("scope", graphTokenScope)
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
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return "", readErr
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("defender graph token: http %d: %s", resp.StatusCode, defenderAPIError(body))
	}
	var tokBody tokenResponse
	if err := json.Unmarshal(body, &tokBody); err != nil {
		return "", err
	}
	if tokBody.AccessToken == "" {
		return "", fmt.Errorf("defender graph token: empty access_token")
	}
	c.graphToken = tokBody.AccessToken
	if tokBody.ExpiresIn > 0 {
		c.graphTokenExp = time.Now().Add(time.Duration(tokBody.ExpiresIn) * time.Second)
	} else {
		c.graphTokenExp = time.Now().Add(50 * time.Minute)
	}
	return c.graphToken, nil
}

func normalizeGUID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.TrimPrefix(id, "{")
	id = strings.TrimSuffix(id, "}")
	return id
}

func odataQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
