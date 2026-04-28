package logic

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gravitl/netmaker/models"
)

func TestGetEgressPresetByID_GitHub(t *testing.T) {
	p, ok := GetEgressPresetByID("github")
	if !ok {
		t.Fatal("expected github preset")
	}
	if p.SuggestedDomain == "" {
		t.Fatal("expected suggested domain")
	}
	if !PresetYieldsStaticDomainAns(p) {
		t.Fatal("github should yield static CIDRs")
	}
}

func TestApplyEgressPresetToEgressReq(t *testing.T) {
	req := &models.EgressReq{PresetID: "github"}
	if err := ApplyEgressPresetToEgressReq(req); err != nil {
		t.Fatal(err)
	}
	if req.Name == "" || req.Domain == "" {
		t.Fatalf("expected name and domain filled: %#v", req)
	}
}

func TestApplyEgressPreset_UnknownID(t *testing.T) {
	req := &models.EgressReq{PresetID: "no-such-preset"}
	err := ApplyEgressPresetToEgressReq(req)
	if err == nil {
		t.Fatal("expected error")
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

func TestResolveGitHubMetaFromFixture(t *testing.T) {
	path := filepath.Join("testdata", "github-meta-sample.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skip("fixture missing")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(b)
	}))
	t.Cleanup(srv.Close)
	orig := gitHubMetaURL
	gitHubMetaURL = srv.URL
	t.Cleanup(func() { gitHubMetaURL = orig })
	cidrs, err := resolveGitHubMetaCIDRs(srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if len(cidrs) == 0 {
		t.Fatal("expected cidr list")
	}
}
