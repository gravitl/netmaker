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

func TestConfiguredDomainsForEgress_legacyDomainOnly(t *testing.T) {
	e := schema.Egress{Domain: "z.example.com"}
	got := ConfiguredDomainsForEgress(e)
	if len(got) != 1 || got[0] != "z.example.com" {
		t.Fatalf("got %v", got)
	}
}

func TestConfiguredDomainsForEgress_domainsJSONWins(t *testing.T) {
	e := schema.Egress{
		Domain:  "legacy.only",
		Domains: datatypes.JSONSlice[string]{"a.example.com", "b.example.com"},
	}
	got := ConfiguredDomainsForEgress(e)
	if !slices.Equal(got, []string{"a.example.com", "b.example.com"}) {
		t.Fatalf("got %v", got)
	}
}
