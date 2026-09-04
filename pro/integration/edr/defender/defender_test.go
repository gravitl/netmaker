package defender

import (
	"testing"
)

func TestNormalizeMachine_OnboardedHealthy(t *testing.T) {
	got := normalizeMachine(securityMachine{
		ID:               "machine-1",
		ComputerDNSName:  "WIN-PC",
		OnboardingStatus: "onboarded",
		HealthStatus:     "active",
		RiskScore:        "low",
	})
	if !got.AgentInstalled || !got.AgentHealthy {
		t.Fatalf("expected installed healthy agent, got installed=%v healthy=%v", got.AgentInstalled, got.AgentHealthy)
	}
	if got.RiskLevel != "low" {
		t.Fatalf("unexpected risk %q", got.RiskLevel)
	}
}

func TestNormalizeMachine_InactiveNotHealthy(t *testing.T) {
	got := normalizeMachine(securityMachine{
		OnboardingStatus: "onboarded",
		HealthStatus:     "Inactive",
	})
	if got.AgentHealthy {
		t.Fatalf("expected inactive machine to be unhealthy, got healthy=%v", got.AgentHealthy)
	}
}
