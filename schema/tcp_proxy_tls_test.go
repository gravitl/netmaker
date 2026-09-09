package schema_test

import (
	"testing"

	"github.com/gravitl/netmaker/schema"
)

func TestNormaliseTcpProxyTLSMode(t *testing.T) {
	got, err := schema.NormaliseTcpProxyTLSMode("")
	if err != nil || got != schema.TcpProxyTLSModeSelfSigned {
		t.Fatalf("got %q %v", got, err)
	}
	got, err = schema.NormaliseTcpProxyTLSMode("proxy")
	if err != nil || got != schema.TcpProxyTLSModeProxy {
		t.Fatalf("got %q %v", got, err)
	}
	_, err = schema.NormaliseTcpProxyTLSMode("letsencrypt")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNormaliseTcpProxyPublicHostname(t *testing.T) {
	got, err := schema.NormaliseTcpProxyPublicHostname("")
	if err != nil || got != "" {
		t.Fatalf("empty: %q %v", got, err)
	}
	got, err = schema.NormaliseTcpProxyPublicHostname("wss://gateway.example.com/uplink/v1")
	if err != nil || got != "gateway.example.com" {
		t.Fatalf("url: %q %v", got, err)
	}
	got, err = schema.NormaliseTcpProxyPublicHostname("gateway.example.com:8443")
	if err != nil || got != "gateway.example.com" {
		t.Fatalf("hostport: %q %v", got, err)
	}
}
