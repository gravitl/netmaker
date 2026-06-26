package wazuh

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	edrpkg "github.com/gravitl/netmaker/pro/integration/edr"
)

func TestNormalizeAgent_ActiveHealthy(t *testing.T) {
	got := normalizeAgent(wazuhAgent{
		ID:     "001",
		Name:   "web-01",
		Status: "active",
	})
	if !got.AgentHealthy || got.RiskLevel != "none" {
		t.Fatalf("expected active healthy, got healthy=%v risk=%q", got.AgentHealthy, got.RiskLevel)
	}
}

func TestNormalizeAgent_DisconnectedRisk(t *testing.T) {
	got := normalizeAgent(wazuhAgent{
		ID:     "002",
		Status: "disconnected",
	})
	if got.AgentHealthy || got.RiskLevel != "medium" {
		t.Fatalf("expected disconnected medium risk, got healthy=%v risk=%q", got.AgentHealthy, got.RiskLevel)
	}
}

func TestVerify_Authenticates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != authPath {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(apiEnvelope{
			Data:  json.RawMessage(`{"token":"tok"}`),
			Error: 0,
		})
	}))
	defer srv.Close()

	c := &Client{
		baseURL:  srv.URL,
		username: "wazuh",
		password: "wazuh",
		http:     srv.Client(),
	}
	if err := c.Verify(context.Background()); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestNew_SkipTLSVerifyAlias(t *testing.T) {
	p, err := New([]byte(`{
		"manager_url": "https://wazuh.example.com:55000",
		"username": "wazuh-wui",
		"password": "secret",
		"skip_tls_verify": true
	}`))
	if err != nil {
		t.Fatal(err)
	}
	c := p.(*Client)
	if c.http.Transport == nil {
		t.Fatal("expected custom transport when skip_tls_verify is true")
	}
}

func TestLookupBySerial_UsesHardwareAndAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == authPath:
			_ = json.NewEncoder(w).Encode(apiEnvelope{
				Data:  json.RawMessage(`{"token":"tok"}`),
				Error: 0,
			})
		case r.URL.Path == hardwarePath:
			if r.URL.Query().Get("board_serial") != "SN-42" {
				t.Fatalf("board_serial = %q", r.URL.Query().Get("board_serial"))
			}
			_ = json.NewEncoder(w).Encode(apiEnvelope{
				Data: json.RawMessage(`{
					"affected_items":[{"agent_id":"001","board_serial":"SN-42"}],
					"total_affected_items":1
				}`),
				Error: 0,
			})
		case r.URL.Path == agentsPath:
			_ = json.NewEncoder(w).Encode(apiEnvelope{
				Data: json.RawMessage(`{
					"affected_items":[{"id":"001","name":"web-01","status":"active"}],
					"total_affected_items":1
				}`),
				Error: 0,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := &Client{
		baseURL:  srv.URL,
		username: "wazuh",
		password: "wazuh",
		http:     srv.Client(),
	}
	ep, err := c.LookupBySerial(context.Background(), "SN-42")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if ep.SerialNumber != "SN-42" || ep.ProviderDeviceID != "001" {
		t.Fatalf("unexpected endpoint: %+v", ep)
	}
}

func TestLookupBySerial_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case authPath:
			_ = json.NewEncoder(w).Encode(apiEnvelope{
				Data:  json.RawMessage(`{"token":"tok"}`),
				Error: 0,
			})
		case hardwarePath:
			_ = json.NewEncoder(w).Encode(apiEnvelope{
				Data: json.RawMessage(`{
					"affected_items":[],
					"total_affected_items":0
				}`),
				Error: 0,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := &Client{
		baseURL:  srv.URL,
		username: "wazuh",
		password: "wazuh",
		http:     srv.Client(),
	}
	_, err := c.LookupBySerial(context.Background(), "missing")
	if err == nil || !strings.Contains(err.Error(), edrpkg.ErrDeviceNotFoundInEDR.Error()) {
		t.Fatalf("expected not found, got %v", err)
	}
}
