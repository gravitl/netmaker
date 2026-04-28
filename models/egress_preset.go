package models

// EgressPresetApp is a catalog entry for domain-based egress (UI + optional apply by preset_id).
type EgressPresetApp struct {
	Name            string   `json:"name"`
	ID              string   `json:"id"`
	Description     string   `json:"description,omitempty"`
	Sources         []string `json:"sources"`
	Domains         []string `json:"domains"`
	Routes          []string `json:"routes,omitempty"`
	Group           string   `json:"group"`
	SuggestedDomain string   `json:"suggestedDomain"`
}
