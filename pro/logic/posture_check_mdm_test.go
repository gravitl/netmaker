package logic

import (
	"testing"

	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func TestMergePostureCheckUpdatePreservesMDMConfig(t *testing.T) {
	existing := &schema.PostureCheck{
		Attribute: schema.MDMCompliance,
		Config: datatypes.JSONMap{
			"require_enrolled":    true,
			"require_compliant":   false,
			"max_state_age_hours": 24,
		},
	}
	update := &schema.PostureCheck{
		Attribute: schema.MDMCompliance,
		Status:    false,
	}

	MergePostureCheckUpdate(existing, update)

	if update.Config == nil {
		t.Fatal("expected config to be merged from existing check")
	}
	if !asBool(update.Config["require_enrolled"]) {
		t.Fatal("expected require_enrolled to be preserved")
	}
	if asBool(update.Config["require_compliant"]) {
		t.Fatal("expected require_compliant to remain false")
	}
	if asInt(update.Config["max_state_age_hours"]) != 24 {
		t.Fatalf("expected max_state_age_hours 24, got %d", asInt(update.Config["max_state_age_hours"]))
	}
}

func TestMergePostureCheckUpdateOverlayConfig(t *testing.T) {
	existing := &schema.PostureCheck{
		Attribute: schema.MDMCompliance,
		Config: datatypes.JSONMap{
			"require_enrolled":    true,
			"require_compliant":   true,
			"max_state_age_hours": 24,
		},
	}
	update := &schema.PostureCheck{
		Attribute: schema.MDMCompliance,
		Config: datatypes.JSONMap{
			"require_compliant": false,
		},
	}

	MergePostureCheckUpdate(existing, update)

	if !asBool(update.Config["require_enrolled"]) {
		t.Fatal("expected require_enrolled from existing config")
	}
	if asBool(update.Config["require_compliant"]) {
		t.Fatal("expected require_compliant to be overridden by update")
	}
}
