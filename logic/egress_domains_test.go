package logic

import (
	"slices"
	"testing"

	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func TestIsEgressDomainPattern(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"api.example.com", true},
		{"*.example.com", true},
		{"*.api.github.com", true},
		{"*", false},
		{"not-a-fqdn", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsEgressDomainPattern(tt.in); got != tt.want {
			t.Errorf("IsEgressDomainPattern(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeEgressReqDomains_primaryFirstAndDedupe(t *testing.T) {
	got, err := NormalizeEgressReqDomains([]string{"github.com", "*.github.com", "api.github.com", "github.com"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"github.com", "*.github.com", "api.github.com"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNormalizeEgressReqDomains_invalidWildcard(t *testing.T) {
	_, err := NormalizeEgressReqDomains([]string{"*.invalid"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEgressDomainsEqual(t *testing.T) {
	if !EgressDomainsEqual([]string{"b", "a"}, []string{"a", "b"}) {
		t.Fatal("expected equal as sets")
	}
	if EgressDomainsEqual([]string{"a"}, []string{"a", "b"}) {
		t.Fatal("expected not equal")
	}
}

func TestConfiguredDomainsForEgress(t *testing.T) {
	e := schema.Egress{Domains: datatypes.JSONSlice[string]{"a.example.com", "b.example.com"}}
	got := ConfiguredDomainsForEgress(e)
	if !slices.Equal(got, []string{"a.example.com", "b.example.com"}) {
		t.Fatalf("got %v", got)
	}
	e = schema.Egress{}
	if got := ConfiguredDomainsForEgress(e); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestApplyConfiguredDomainsToEgress(t *testing.T) {
	var e schema.Egress
	ApplyConfiguredDomainsToEgress(&e, []string{"a.example.com", "b.example.com"})
	if !slices.Equal([]string(e.Domains), []string{"a.example.com", "b.example.com"}) {
		t.Fatalf("domains got %v", e.Domains)
	}
	ApplyConfiguredDomainsToEgress(&e, nil)
	if len(e.Domains) != 0 {
		t.Fatalf("clear got domains=%v", e.Domains)
	}
}

func TestNormalizeEgressDomain_preservesFullHostname(t *testing.T) {
	if got := normalizeEgressDomain("oauth2.googleapis.com"); got != "oauth2.googleapis.com" {
		t.Fatalf("got %q, want oauth2.googleapis.com", got)
	}
	if got := normalizeEgressDomain("  OAuth2.Googleapis.COM  "); got != "oauth2.googleapis.com" {
		t.Fatalf("got %q, want lowercased full hostname", got)
	}
}
