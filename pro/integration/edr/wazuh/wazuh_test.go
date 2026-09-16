package wazuh

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	edrpkg "github.com/gravitl/netmaker/pro/integration/edr"
	"github.com/gravitl/netmaker/schema"
)

func TestNormalizeAgent_ActiveHealthy(t *testing.T) {
	got := normalizeAgent(wazuhAgent{
		ID:     "001",
		Name:   "web-01",
		Status: "active",
		IP:     "10.0.0.5",
	})
	if !got.AgentHealthy || got.RiskLevel != "none" {
		t.Fatalf("expected active healthy, got healthy=%v risk=%q", got.AgentHealthy, got.RiskLevel)
	}
	if got.EndpointIP != "10.0.0.5" {
		t.Fatalf("expected endpoint ip on managed endpoint, got %q", got.EndpointIP)
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

func TestLookupForHost_BySerialFirst(t *testing.T) {
	hostID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	nameLookupCalled := false
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
			if r.URL.Query().Get("name") != "" {
				nameLookupCalled = true
			}
			if r.URL.Query().Get("agents_list") != "001" {
				t.Fatalf("agents_list = %q", r.URL.Query().Get("agents_list"))
			}
			_ = json.NewEncoder(w).Encode(apiEnvelope{
				Data: json.RawMessage(`{
					"affected_items":[{"id":"001","name":"other-name","status":"active","ip":"10.0.0.1"}],
					"total_affected_items":1
				}`),
				Error: 0,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, username: "wazuh", password: "wazuh", http: srv.Client()}
	ep, matchedBy, err := c.LookupForHost(context.Background(), schema.Host{
		ID:           hostID,
		SerialNumber: "SN-42",
		Name:         hostID.String(),
	})
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if matchedBy != schema.EDRMatchSerialNumber || ep.ProviderDeviceID != "001" {
		t.Fatalf("unexpected match: by=%q ep=%+v", matchedBy, ep)
	}
	if nameLookupCalled {
		t.Fatal("expected serial lookup to succeed without name-based agent query")
	}
}

func TestLookupForHost_ByHostUUIDName(t *testing.T) {
	hostID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == authPath:
			_ = json.NewEncoder(w).Encode(apiEnvelope{
				Data:  json.RawMessage(`{"token":"tok"}`),
				Error: 0,
			})
		case r.URL.Path == agentsPath:
			if r.URL.Query().Get("name") != hostID.String() {
				t.Fatalf("name = %q", r.URL.Query().Get("name"))
			}
			_ = json.NewEncoder(w).Encode(apiEnvelope{
				Data: json.RawMessage(`{
					"affected_items":[{"id":"001","name":"` + hostID.String() + `","status":"active","ip":"10.0.0.99"}],
					"total_affected_items":1
				}`),
				Error: 0,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, username: "wazuh", password: "wazuh", http: srv.Client()}
	ep, matchedBy, err := c.LookupForHost(context.Background(), schema.Host{
		ID:         hostID,
		Name:       "web-01",
		EndpointIP: net.ParseIP("203.0.113.1"),
	})
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if matchedBy != schema.EDRMatchHostID || ep.ProviderDeviceID != "001" {
		t.Fatalf("unexpected match: by=%q ep=%+v", matchedBy, ep)
	}
}

func TestLookupForHost_ByHostnameAndEndpointIP(t *testing.T) {
	hostID := uuid.New()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == authPath:
			_ = json.NewEncoder(w).Encode(apiEnvelope{
				Data:  json.RawMessage(`{"token":"tok"}`),
				Error: 0,
			})
		case r.URL.Path == agentsPath:
			name := r.URL.Query().Get("name")
			switch name {
			case hostID.String():
				_ = json.NewEncoder(w).Encode(apiEnvelope{
					Data:  json.RawMessage(`{"affected_items":[],"total_affected_items":0}`),
					Error: 0,
				})
			case "web-01":
				_ = json.NewEncoder(w).Encode(apiEnvelope{
					Data: json.RawMessage(`{
						"affected_items":[{"id":"002","name":"web-01","status":"active","ip":"10.0.0.5"}],
						"total_affected_items":1
					}`),
					Error: 0,
				})
			default:
				t.Fatalf("unexpected name query %q", name)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, username: "wazuh", password: "wazuh", http: srv.Client()}
	ep, matchedBy, err := c.LookupForHost(context.Background(), schema.Host{
		ID:         hostID,
		Name:       "web-01",
		EndpointIP: net.ParseIP("10.0.0.5"),
	})
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if matchedBy != schema.EDRMatchHostname || ep.ProviderDeviceID != "002" {
		t.Fatalf("unexpected match: by=%q ep=%+v", matchedBy, ep)
	}
}

func TestLookupForHost_HostnameOnlyRejected(t *testing.T) {
	hostID := uuid.New()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == authPath:
			_ = json.NewEncoder(w).Encode(apiEnvelope{
				Data:  json.RawMessage(`{"token":"tok"}`),
				Error: 0,
			})
		case r.URL.Path == agentsPath:
			_ = json.NewEncoder(w).Encode(apiEnvelope{
				Data: json.RawMessage(`{
					"affected_items":[{"id":"002","name":"web-01","status":"active","ip":"10.0.0.9"}],
					"total_affected_items":1
				}`),
				Error: 0,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, username: "wazuh", password: "wazuh", http: srv.Client()}
	_, _, err := c.LookupForHost(context.Background(), schema.Host{
		ID:         hostID,
		Name:       "web-01",
		EndpointIP: net.ParseIP("10.0.0.5"),
	})
	if err == nil || !strings.Contains(err.Error(), edrpkg.ErrDeviceNotFoundInEDR.Error()) {
		t.Fatalf("expected not found, got %v", err)
	}
}
