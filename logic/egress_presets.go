package logic

import (
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

// egressPreset catalog is built once in init (see egress_presets_catalog.go).
var (
	egressPresetList      []models.EgressPresetApp
	egressPresetByID      map[string]models.EgressPresetApp
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

// ErrVirtualNATNotForEgressApps is returned when virtual NAT is requested for a preset egress app.
var ErrVirtualNATNotForEgressApps = errors.New("virtual NAT is not supported for egress apps")

// IsEgressAppEgress reports whether the egress was created from a catalog preset (egress app).
func IsEgressAppEgress(e schema.Egress) bool {
	return strings.TrimSpace(e.PresetID) != ""
}

// ValidateEgressAppNATMode rejects virtual NAT for preset-based egress apps.
func ValidateEgressAppNATMode(e schema.Egress) error {
	if IsEgressAppEgress(e) && e.Mode == schema.VirtualNAT {
		return ErrVirtualNATNotForEgressApps
	}
	return nil
}

// ApplyEgressPresetToEgressReq merges catalog defaults into req. Rules: explicit non-empty
// name, description, and domains in req override preset. PresetID must already be a known id.
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
	if len(req.Domains) == 0 {
		trimEgressPresetDomains(&p)
		norm, err := NormalizeEgressReqDomains(p.Domains)
		if err != nil {
			return err
		}
		if len(norm) > 0 {
			req.Domains = norm
		} else {
			if p.SuggestedDomain == "" {
				enrichSuggestedDomain(&p)
			}
			if sd := strings.TrimSpace(strings.ToLower(p.SuggestedDomain)); sd != "" {
				req.Domains = []string{sd}
			}
		}
	}
	return nil
}

func trimEgressPresetDomains(p *models.EgressPresetApp) {
	var out []string
	for _, d := range p.Domains {
		d = strings.TrimSpace(d)
		if d == "" || strings.HasPrefix(d, "*.") {
			continue
		}
		out = append(out, d)
	}
	p.Domains = out
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

// IsAWSEgressPreset reports whether presetID refers to an AWS catalog entry.
func IsAWSEgressPreset(presetID string) bool {
	return strings.HasPrefix(presetID, "aws-")
}

// PresetYieldsAWSIPRanges reports whether the preset is backed by AWS ip-ranges.json.
func PresetYieldsAWSIPRanges(p models.EgressPresetApp) bool {
	for _, src := range p.Sources {
		src = strings.TrimSpace(src)
		if strings.HasPrefix(src, "https://ip-ranges.amazonaws.com/") {
			return true
		}
	}
	return false
}

// ResolveAWSEgressPresetCIDRs fetches public AWS CIDR data for a supported AWS preset.
func ResolveAWSEgressPresetCIDRs(client *http.Client, p models.EgressPresetApp) ([]string, error) {
	if !PresetYieldsAWSIPRanges(p) {
		return nil, nil
	}
	return resolveAWSPresetCIDRs(client, p)
}
