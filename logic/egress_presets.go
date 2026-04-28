package logic

import (
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/gravitl/netmaker/models"
)

// egressPreset catalog is built once in init (see egress_presets_catalog.go).
var (
	egressPresetList     []models.EgressPresetApp
	egressPresetByID     map[string]models.EgressPresetApp
	egressPresetIndexOnce sync.Once
)

func buildEgressPresetIndex() {
	egressPresetByID = make(map[string]models.EgressPresetApp, len(egressPresetList))
	for i := range egressPresetList {
		enrichSuggestedDomain(&egressPresetList[i])
		egressPresetByID[egressPresetList[i].ID] = egressPresetList[i]
	}
}

// ListEgressPresets returns the static egress preset catalog (defensive copy of slice header; entries are values).
func ListEgressPresets() []models.EgressPresetApp {
	egressPresetIndexOnce.Do(func() {
		egressPresetList = buildEgressPresetCatalog()
		buildEgressPresetIndex()
	})
	out := make([]models.EgressPresetApp, len(egressPresetList))
	copy(out, egressPresetList)
	return out
}

// GetEgressPresetByID returns a catalog entry by id.
func GetEgressPresetByID(id string) (models.EgressPresetApp, bool) {
	egressPresetIndexOnce.Do(func() {
		egressPresetList = buildEgressPresetCatalog()
		buildEgressPresetIndex()
	})
	if id == "" {
		return models.EgressPresetApp{}, false
	}
	p, ok := egressPresetByID[id]
	return p, ok
}

// ErrUnknownEgressPreset is returned when preset_id does not match the catalog.
var ErrUnknownEgressPreset = errors.New("unknown egress preset_id")

// ApplyEgressPresetToEgressReq merges catalog defaults into req. Rules: explicit non-empty name, description, and domain in req override preset. PresetID must already be a known id.
func ApplyEgressPresetToEgressReq(req *models.EgressReq) error {
	if req == nil || req.PresetID == "" {
		return nil
	}
	p, ok := GetEgressPresetByID(req.PresetID)
	if !ok {
		return ErrUnknownEgressPreset
	}
	trimEgressPresetDomains(&p)
	if req.Name == "" {
		req.Name = p.Name
	}
	if req.Description == "" && p.Description != "" {
		req.Description = p.Description
	}
	if req.Domain == "" {
		if p.SuggestedDomain == "" {
			enrichSuggestedDomain(&p)
		}
		req.Domain = strings.TrimSpace(p.SuggestedDomain)
	}
	return nil
}

func trimEgressPresetDomains(p *models.EgressPresetApp) {
	for i := range p.Domains {
		p.Domains[i] = strings.TrimSpace(p.Domains[i])
	}
}

func enrichSuggestedDomain(p *models.EgressPresetApp) {
	if p == nil || p.SuggestedDomain != "" {
		return
	}
	for _, d := range p.Domains {
		d = strings.TrimSpace(d)
		d = strings.TrimPrefix(d, "*.")
		if d != "" && IsFQDN(d) {
			p.SuggestedDomain = d
			return
		}
	}
}

// PresetYieldsStaticDomainAns returns true if this preset can resolve CIDRs server-side (no dependency on help/docs-only sources).
func PresetYieldsStaticDomainAns(p models.EgressPresetApp) bool {
	trimEgressPresetDomains(&p)
	for _, src := range p.Sources {
		src = strings.TrimSpace(src)
		if isFetchableCIDRSource(src) {
			return true
		}
	}
	return false
}

func isFetchableCIDRSource(u string) bool {
	switch {
	case strings.HasPrefix(u, "https://ip-ranges.amazonaws.com/"):
		return true
	case u == "https://api.github.com/meta":
		return true
	case strings.HasPrefix(u, "https://api.fastly.com/"):
		return true
	}
	return false
}

// ResolveEgressPresetCIDRs fetches public CIDR data when the preset is backed by a supported first fetchable source.
func ResolveEgressPresetCIDRs(client *http.Client, p models.EgressPresetApp) (cidrs []string, err error) {
	trimEgressPresetDomains(&p)
	for _, src := range p.Sources {
		src = strings.TrimSpace(src)
		if !isFetchableCIDRSource(src) {
			continue
		}
		if strings.HasPrefix(src, "https://ip-ranges.amazonaws.com/") {
			return resolveAWSPresetCIDRs(client, p)
		}
		if src == "https://api.github.com/meta" {
			return resolveGitHubMetaCIDRs(client)
		}
		if strings.HasPrefix(src, "https://api.fastly.com/") {
			return resolveFastlyPublicCIDRs(client, src)
		}
	}
	return nil, nil
}
