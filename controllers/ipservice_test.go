package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseIPTrimsForwardedForEntries(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.Header.Set("X-FORWARDED-FOR", "10.0.0.1, 8.8.8.8")
	req.RemoteAddr = "10.0.0.2:1234"

	ip, err := parseIP(req)
	if err != nil {
		t.Fatalf("parseIP() returned error: %v", err)
	}
	if ip != "8.8.8.8" {
		t.Fatalf("parseIP() = %q, want %q", ip, "8.8.8.8")
	}
}
