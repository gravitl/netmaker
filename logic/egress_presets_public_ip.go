package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// gitHubMetaURL is the GitHub /meta API (overridable in tests).
var gitHubMetaURL = "https://api.github.com/meta"

type gitHubMeta struct {
	Web     []string `json:"web"`
	API     []string `json:"api"`
	Git     []string `json:"git"`
	Pages   []string `json:"pages"`
	Actions []string `json:"actions"`
	Dependabot []string `json:"dependabot"`
}

func resolveGitHubMetaCIDRs(client *http.Client) ([]string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequest(http.MethodGet, gitHubMetaURL, nil)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	req = req.WithContext(ctx)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github meta: status %d", res.StatusCode)
	}
	var m gitHubMeta
	if err := json.NewDecoder(res.Body).Decode(&m); err != nil {
		return nil, err
	}
	var out []string
	for _, b := range [][]string{m.Web, m.API, m.Git, m.Pages, m.Actions, m.Dependabot} {
		out = append(out, b...)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("github meta: no CIDR blocks")
	}
	return UniqueIPNetStrList(out), nil
}

func resolveFastlyPublicCIDRs(client *http.Client, url string) ([]string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if url == "" {
		url = "https://api.fastly.com/public-ip-list"
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	req = req.WithContext(ctx)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fastly public-ip-list: status %d", res.StatusCode)
	}
	var f struct {
		Addresses     []string `json:"addresses"`
		IPv4Addresses []string `json:"ipv4_addresses"`
		IPv6Addresses []string `json:"ipv6_addresses"`
	}
	if err := json.NewDecoder(res.Body).Decode(&f); err != nil {
		return nil, err
	}
	var out []string
	out = append(out, f.Addresses...)
	out = append(out, f.IPv4Addresses...)
	out = append(out, f.IPv6Addresses...)
	if len(out) == 0 {
		return nil, fmt.Errorf("fastly: no ip entries in response")
	}
	return UniqueIPNetStrList(out), nil
}
