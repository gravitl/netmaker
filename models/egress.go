package models

import "github.com/gravitl/netmaker/schema"

type EgressReq struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Network     string         `json:"network"`
	Description string         `json:"description"`
	Nodes       map[string]int `json:"nodes"`
	Tags        map[string]int `json:"tags"`
	Range       string         `json:"range"`
	// Domains optional list of logical hostnames for domain-based egress (exact or *.example.com).
	Domains  []string             `json:"domains"`
	Nat      bool                 `json:"nat"`
	Mode     schema.EgressNATMode `json:"mode"`
	Status   bool                 `json:"status"`
	IsInetGw bool                 `json:"is_internet_gateway"`
	// PresetID optional: reference to a catalog preset (see GET /api/v1/egress/presets). Explicit name/domain in the body override preset defaults.
	PresetID string `json:"preset_id"`
}
