package crowdstrike

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	edrpkg "github.com/gravitl/netmaker/pro/integration/edr"
)

func TestNormalizeDevice_ContainedCritical(t *testing.T) {
	got := normalizeDevice(falconDevice{
		DeviceID:     "dev-1",
		SerialNumber: "SN1",
		Hostname:     "host-1",
		Status:       "contained",
	})
	if got.RiskLevel != "critical" || !got.Contained || got.AgentHealthy {
		t.Fatalf("expected contained critical unhealthy, got risk=%q contained=%v healthy=%v",
			got.RiskLevel, got.Contained, got.AgentHealthy)
	}
}

func TestNormalizeDevice_NormalHealthy(t *testing.T) {
	got := normalizeDevice(falconDevice{
		DeviceID:     "dev-1",
		SerialNumber: "SN1",
		Status:       "normal",
	})
	if got.RiskLevel != "none" || got.Contained || !got.AgentHealthy {
		t.Fatalf("expected normal healthy, got risk=%q contained=%v healthy=%v",
			got.RiskLevel, got.Contained, got.AgentHealthy)
	}
}

func TestSearchDeviceBySerial_URLEncoding(t *testing.T) {
	filter := url.QueryEscape("serial_number:'SN-42'")
	if filter != "serial_number%3A%27SN-42%27" {
		t.Fatalf("encoded filter = %q", filter)
	}
}

func TestLookupBySerial_UsesFilterQuery(t *testing.T) {
	var gotFilter string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == tokenPath:
			_ = json.NewEncoder(w).Encode(tokenResponse{AccessToken: "tok", ExpiresIn: 3600})
		case r.URL.Path == queryPath:
			gotFilter = r.URL.Query().Get("filter")
			_ = json.NewEncoder(w).Encode(queryResponse{Resources: []string{"aid-1"}})
		case r.URL.Path == entitiesPath:
			_ = json.NewEncoder(w).Encode(entitiesResponse{
				Resources: []falconDevice{{
					DeviceID:     "aid-1",
					SerialNumber: "SN-42",
					Hostname:     "host-1",
					Status:       "normal",
				}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := &Client{
		baseURL:      srv.URL,
		clientID:     "id",
		clientSecret: "secret",
		http:         srv.Client(),
	}
	ep, err := c.LookupBySerial(context.Background(), "SN-42")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if gotFilter != "serial_number:'SN-42'" {
		t.Fatalf("filter = %q", gotFilter)
	}
	if ep.SerialNumber != "SN-42" || ep.ProviderDeviceID != "aid-1" {
		t.Fatalf("unexpected endpoint: %+v", ep)
	}
}

func TestLookupBySerial_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case tokenPath:
			_ = json.NewEncoder(w).Encode(tokenResponse{AccessToken: "tok", ExpiresIn: 3600})
		case queryPath:
			_, _ = w.Write([]byte(`{
				"meta": {"pagination": {"offset": 0, "limit": 100, "total": 0}},
				"resources": [],
				"errors": []
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := &Client{
		baseURL:      srv.URL,
		clientID:     "id",
		clientSecret: "secret",
		http:         srv.Client(),
	}
	_, err := c.LookupBySerial(context.Background(), "missing")
	if err == nil || !strings.Contains(err.Error(), edrpkg.ErrDeviceNotFoundInEDR.Error()) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestQueryResponse_NumericOffset(t *testing.T) {
	body := `{"meta":{"pagination":{"offset":0,"next":""}},"resources":[]}`
	var page queryResponse
	if err := json.Unmarshal([]byte(body), &page); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if page.Meta.Pagination.Offset.String() != "0" {
		t.Fatalf("offset = %q", page.Meta.Pagination.Offset.String())
	}
}
