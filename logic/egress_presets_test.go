package logic

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gravitl/netmaker/models"
)

func TestEgressPresetExtras_AllHaveUsableDomains(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("egress_preset_extras.json"))
	if err != nil {
		t.Fatal(err)
	}
	var extras []models.EgressPresetApp
	if err := json.Unmarshal(b, &extras); err != nil {
		t.Fatal(err)
	}
	for _, p := range extras {
		trimEgressPresetDomains(&p)
		if len(p.Domains) == 0 {
			t.Errorf("preset %q (%s) has no domains after trim", p.ID, p.Name)
			continue
		}
		for _, d := range p.Domains {
			if strings.HasPrefix(d, "*.") {
				t.Errorf("preset %q: wildcard domain %q is stripped at runtime; use apex FQDN", p.ID, d)
			}
			if !IsEgressDomainPattern(d) {
				t.Errorf("preset %q: invalid domain %q", p.ID, d)
			}
		}
	}
}

func TestGetEgressPresetByID_GitHub(t *testing.T) {
	p, ok := GetEgressPresetByID("github")
	if !ok {
		t.Fatal("expected github preset")
	}
	if p.SuggestedDomain == "" {
		t.Fatal("expected suggested domain")
	}
	if len(p.Sources) == 0 {
		t.Fatal("expected catalog source metadata")
	}
}

func TestApplyEgressPresetToEgressReq(t *testing.T) {
	req := &models.EgressReq{PresetID: "github"}
	if err := ApplyEgressPresetToEgressReq(req); err != nil {
		t.Fatal(err)
	}
	if req.Name == "" || len(req.Domains) == 0 {
		t.Fatalf("expected name and domains filled: %#v", req)
	}
}

func TestGetEgressPresetByID_JiraIncludesAtlassianDomains(t *testing.T) {
	p, ok := GetEgressPresetByID("jira")
	if !ok {
		t.Fatal("expected jira preset")
	}
	domains := make(map[string]struct{}, len(p.Domains))
	for _, d := range p.Domains {
		domains[d] = struct{}{}
	}
	for _, want := range []string{"atlassian.net", "atlassian.com", "jira.com", "recaptcha.net"} {
		if _, ok := domains[want]; !ok {
			t.Fatalf("expected domain %q in jira preset, got %v", want, p.Domains)
		}
	}
}

func TestApplyEgressPresetToEgressReq_SparsePresets(t *testing.T) {
	for _, id := range []string{"slack", "hubspot", "salesforce", "zoom"} {
		req := &models.EgressReq{PresetID: id}
		if err := ApplyEgressPresetToEgressReq(req); err != nil {
			t.Fatalf("preset %s: %v", id, err)
		}
		if len(req.Domains) < 3 {
			t.Fatalf("preset %s: expected multiple domains, got %v", id, req.Domains)
		}
	}
}

func TestGetEgressPresetByID_OktaIncludesRequiredDomains(t *testing.T) {
	p, ok := GetEgressPresetByID("okta")
	if !ok {
		t.Fatal("expected okta preset")
	}
	domains := make(map[string]struct{}, len(p.Domains))
	for _, d := range p.Domains {
		domains[d] = struct{}{}
	}
	for _, want := range []string{"okta.com", "oktacdn.com", "oktapreview.com", "ocsp.digicert.com"} {
		if _, ok := domains[want]; !ok {
			t.Fatalf("expected domain %q in okta preset, got %v", want, p.Domains)
		}
	}
}

func TestApplyEgressPreset_UnknownID(t *testing.T) {
	req := &models.EgressReq{PresetID: "no-such-preset"}
	err := ApplyEgressPresetToEgressReq(req)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestIsAWSEgressPreset(t *testing.T) {
	if !IsAWSEgressPreset("aws-s3-us-east-1") {
		t.Fatal("expected aws preset id")
	}
	if IsAWSEgressPreset("github") {
		t.Fatal("github is not an aws preset")
	}
}

func TestPresetYieldsAWSIPRanges(t *testing.T) {
	p, ok := GetEgressPresetByID("aws-s3-us-east-1")
	if !ok {
		t.Fatal("expected aws preset")
	}
	if !PresetYieldsAWSIPRanges(p) {
		t.Fatal("expected aws ip ranges source")
	}
	p, ok = GetEgressPresetByID("github")
	if !ok {
		t.Fatal("expected github preset")
	}
	if PresetYieldsAWSIPRanges(p) {
		t.Fatal("github preset should not yield aws ip ranges")
	}
}

func TestApplyEgressPresetToEgressReq_AWS(t *testing.T) {
	req := &models.EgressReq{PresetID: "aws-s3-us-east-1"}
	if err := ApplyEgressPresetToEgressReq(req); err != nil {
		t.Fatal(err)
	}
	if req.Name == "" || len(req.Domains) == 0 {
		t.Fatalf("expected name and domains filled: %#v", req)
	}
}

func TestResolveAWSEgressPresetCIDRsFromFixture(t *testing.T) {
	p, ok := GetEgressPresetByID("aws-s3-us-east-1")
	if !ok {
		t.Fatal("expected aws s3 us-east-1")
	}
	path := filepath.Join("testdata", "aws-ip-ranges-sample.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skip("fixture missing")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(b)
	}))
	t.Cleanup(srv.Close)
	client := srv.Client()
	orig := awsIPRangesURL
	awsIPRangesURL = srv.URL
	t.Cleanup(func() { awsIPRangesURL = orig })
	cidrs, err := ResolveAWSEgressPresetCIDRs(client, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cidrs) == 0 {
		t.Fatal("expected cidr list")
	}
}

func TestResolveAWSPresetCIDRsFromFixture(t *testing.T) {
	p, ok := GetEgressPresetByID("aws-s3-us-east-1")
	if !ok {
		t.Fatal("expected aws s3 us-east-1")
	}
	path := filepath.Join("testdata", "aws-ip-ranges-sample.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skip("fixture missing")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(b)
	}))
	t.Cleanup(srv.Close)
	client := srv.Client()
	orig := awsIPRangesURL
	awsIPRangesURL = srv.URL
	t.Cleanup(func() { awsIPRangesURL = orig })
	cidrs, err := resolveAWSPresetCIDRs(client, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cidrs) == 0 {
		t.Fatal("expected cidr list")
	}
}
