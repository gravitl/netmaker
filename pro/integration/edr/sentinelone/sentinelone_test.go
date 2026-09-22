package sentinelone

import "testing"

func TestNormalizeAgent_InfectedCritical(t *testing.T) {
	got := normalizeAgent(s1Agent{
		ID:           "agent-1",
		ComputerName: "mac-1",
		IsActive:     true,
		Infected:     true,
	})
	if got.RiskLevel != "critical" {
		t.Fatalf("expected critical for infected agent, got %q", got.RiskLevel)
	}
}

func TestNormalizeAgent_InfectedWithActiveThreatsCritical(t *testing.T) {
	got := normalizeAgent(s1Agent{
		ID:            "agent-1",
		IsActive:      true,
		Infected:      true,
		ActiveThreats: 3,
	})
	if got.RiskLevel != "critical" {
		t.Fatalf("expected critical for infected agent with threats, got %q", got.RiskLevel)
	}
}

func TestNormalizeAgent_HealthyActiveThreatsHigh(t *testing.T) {
	got := normalizeAgent(s1Agent{
		ID:            "agent-2",
		IsActive:      true,
		ActiveThreats: 2,
	})
	if got.RiskLevel != "high" {
		t.Fatalf("expected high risk with active threats, got %q", got.RiskLevel)
	}
}
