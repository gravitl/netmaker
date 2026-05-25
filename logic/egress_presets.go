package logic

import (
	"errors"
	"strings"
	"sync"

	"github.com/gravitl/netmaker/models"
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
