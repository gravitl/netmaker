package defender

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	edrpkg "github.com/gravitl/netmaker/pro/integration/edr"
	"github.com/gravitl/netmaker/schema"
)

func TestLookupForHost_EntraFilter(t *testing.T) {
	entraID := "32f5f9ec-cd23-41e0-94e8-6b372232ff40"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/oauth2/v2.0/token"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "tok",
				"expires_in":   3600,
			})
		case strings.Contains(r.URL.Path, "/api/machines"):
			if !strings.Contains(r.URL.RawQuery, "aadDeviceId") {
				t.Fatalf("expected aadDeviceId filter, got %q", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"value": []map[string]any{
					{
						"id":               "machine-1",
						"computerDnsName":  "WIN-PC",
						"aadDeviceId":      entraID,
						"onboardingStatus": "onboarded",
						"healthStatus":     "active",
						"riskScore":        "low",
					},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := &Client{
		tenantID:     "tenant",
		clientID:     "client",
		clientSecret: "secret",
		http:         srv.Client(),
	}
	origMachinesURL := machinesURL
	origTokenURLFmt := tokenURLFmt
	machinesURL = srv.URL + "/api/machines"
	tokenURLFmt = srv.URL + "/%s/oauth2/v2.0/token"
	t.Cleanup(func() {
		machinesURL = origMachinesURL
		tokenURLFmt = origTokenURLFmt
	})

	ep, matchedBy, err := c.LookupForHost(context.Background(), schema.Host{
		ID:            uuid.New(),
		EntraDeviceID: entraID,
	})
	if err != nil {
		t.Fatalf("LookupForHost: %v", err)
	}
	if matchedBy != schema.EDRMatchEntraDeviceID {
		t.Fatalf("matchedBy = %q, want entra_device_id", matchedBy)
	}
	if ep.ProviderDeviceID != "machine-1" {
		t.Fatalf("provider device id = %q", ep.ProviderDeviceID)
	}
}

func TestLookupForHost_SerialViaGraph(t *testing.T) {
	entraID := "32f5f9ec-cd23-41e0-94e8-6b372232ff40"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/oauth2/v2.0/token"):
			_ = r.ParseForm()
			scope := r.FormValue("scope")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "tok-" + scope,
				"expires_in":   3600,
			})
		case strings.Contains(r.URL.Path, "/deviceManagement/managedDevices"):
			if !strings.Contains(r.URL.RawQuery, "serialNumber") {
				t.Fatalf("expected serialNumber filter, got %q", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"value": []map[string]any{
					{"azureADDeviceId": entraID},
				},
			})
		case strings.Contains(r.URL.Path, "/api/machines"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"value": []map[string]any{
					{
						"id":               "machine-1",
						"computerDnsName":  "WIN-PC",
						"aadDeviceId":      entraID,
						"onboardingStatus": "onboarded",
						"healthStatus":     "active",
						"riskScore":        "low",
					},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := &Client{
		tenantID:     "tenant",
		clientID:     "client",
		clientSecret: "secret",
		http:         srv.Client(),
	}
	origMachinesURL := machinesURL
	origTokenURLFmt := tokenURLFmt
	origManagedDevicesURL := managedDevicesURL
	machinesURL = srv.URL + "/api/machines"
	managedDevicesURL = srv.URL + "/deviceManagement/managedDevices"
	tokenURLFmt = srv.URL + "/%s/oauth2/v2.0/token"
	t.Cleanup(func() {
		machinesURL = origMachinesURL
		tokenURLFmt = origTokenURLFmt
		managedDevicesURL = origManagedDevicesURL
	})

	ep, matchedBy, err := c.LookupForHost(context.Background(), schema.Host{
		ID:           uuid.New(),
		SerialNumber: "SN-42",
	})
	if err != nil {
		t.Fatalf("LookupForHost: %v", err)
	}
	if matchedBy != schema.EDRMatchSerialNumber {
		t.Fatalf("matchedBy = %q, want serial_number", matchedBy)
	}
	if ep.ProviderDeviceID != "machine-1" {
		t.Fatalf("provider device id = %q", ep.ProviderDeviceID)
	}
}

func TestLookupForHost_EntraNotFoundFallsBackToSerial(t *testing.T) {
	entraID := "32f5f9ec-cd23-41e0-94e8-6b372232ff40"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/oauth2/v2.0/token"):
			_ = r.ParseForm()
			scope := r.FormValue("scope")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "tok-" + scope,
				"expires_in":   3600,
			})
		case strings.Contains(r.URL.Path, "/deviceManagement/managedDevices"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"value": []map[string]any{
					{"azureADDeviceId": entraID},
				},
			})
		case strings.Contains(r.URL.Path, "/api/machines"):
			if strings.Contains(r.URL.RawQuery, entraID) {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"value": []map[string]any{
						{
							"id":               "machine-1",
							"computerDnsName":  "WIN-PC",
							"aadDeviceId":      entraID,
							"onboardingStatus": "onboarded",
							"healthStatus":     "active",
							"riskScore":        "low",
						},
					},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"value": []any{}})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := &Client{
		tenantID:     "tenant",
		clientID:     "client",
		clientSecret: "secret",
		http:         srv.Client(),
	}
	origMachinesURL := machinesURL
	origTokenURLFmt := tokenURLFmt
	origManagedDevicesURL := managedDevicesURL
	machinesURL = srv.URL + "/api/machines"
	managedDevicesURL = srv.URL + "/deviceManagement/managedDevices"
	tokenURLFmt = srv.URL + "/%s/oauth2/v2.0/token"
	t.Cleanup(func() {
		machinesURL = origMachinesURL
		tokenURLFmt = origTokenURLFmt
		managedDevicesURL = origManagedDevicesURL
	})

	ep, matchedBy, err := c.LookupForHost(context.Background(), schema.Host{
		ID:            uuid.New(),
		EntraDeviceID: "wrong-entra-id",
		SerialNumber:  "SN-42",
	})
	if err != nil {
		t.Fatalf("LookupForHost: %v", err)
	}
	if matchedBy != schema.EDRMatchSerialNumber {
		t.Fatalf("matchedBy = %q, want serial_number", matchedBy)
	}
	if ep.ProviderDeviceID != "machine-1" {
		t.Fatalf("provider device id = %q", ep.ProviderDeviceID)
	}
}

func TestLookupForHost_NoIdentifiers(t *testing.T) {
	c := &Client{}
	_, _, err := c.LookupForHost(context.Background(), schema.Host{ID: uuid.New()})
	if err == nil || !strings.Contains(err.Error(), edrpkg.ErrDeviceNotFoundInEDR.Error()) {
		t.Fatalf("expected not found error, got %v", err)
	}
}
