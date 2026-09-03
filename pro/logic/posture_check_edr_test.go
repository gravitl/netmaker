package logic

import (
	"testing"
	"time"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func TestEvaluateEDRCompliance_NoState(t *testing.T) {
	check := schema.PostureCheck{
		Attribute: schema.EDRCompliance,
		Config: datatypes.JSONMap{
			"require_agent_installed": true,
		},
	}
	violated, msg := evaluatePostureCheck(&check, models.PostureCheckDeviceInfo{})
	if !violated || msg != "No EDR status found for this device. It may not be protected or matched yet." {
		t.Fatalf("got violated=%v msg=%q", violated, msg)
	}
}

func TestEvaluateEDRCompliance_RiskExceeded(t *testing.T) {
	check := schema.PostureCheck{
		Attribute: schema.EDRCompliance,
		Config: datatypes.JSONMap{
			"max_allowed_risk_level": "medium",
		},
	}
	d := models.PostureCheckDeviceInfo{
		EDRState: &schema.DeviceEDRState{
			AgentInstalled: true,
			AgentHealthy:   true,
			RiskLevel:      "high",
			LastSyncedAt:   time.Now().UTC(),
		},
	}
	violated, msg := evaluatePostureCheck(&check, d)
	if !violated || msg != "EDR risk level exceeds the allowed maximum." {
		t.Fatalf("got violated=%v msg=%q", violated, msg)
	}
}

func TestEvaluateEDRCompliance_Pass(t *testing.T) {
	check := schema.PostureCheck{
		Attribute: schema.EDRCompliance,
		Config: datatypes.JSONMap{
			"require_agent_installed": true,
			"require_agent_healthy":   true,
			"max_allowed_risk_level":  "medium",
		},
	}
	d := models.PostureCheckDeviceInfo{
		EDRState: &schema.DeviceEDRState{
			AgentInstalled: true,
			AgentHealthy:   true,
			RiskLevel:      "low",
			LastSyncedAt:   time.Now().UTC(),
		},
	}
	violated, msg := evaluatePostureCheck(&check, d)
	if violated {
		t.Fatalf("expected pass, got %q", msg)
	}
}
