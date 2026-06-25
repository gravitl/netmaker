package edr

import "testing"

func TestComputeRiskLevel_ContainedIsCritical(t *testing.T) {
	got := ComputeRiskLevel(VendorSignals{Contained: true})
	if got != RiskCritical {
		t.Fatalf("got %q want critical", got)
	}
}

func TestComputeRiskLevel_IsolatedIsCritical(t *testing.T) {
	got := ComputeRiskLevel(VendorSignals{Isolated: true})
	if got != RiskCritical {
		t.Fatalf("got %q want critical", got)
	}
}

func TestComputeRiskLevel_ActiveMalwareIsHigh(t *testing.T) {
	got := ComputeRiskLevel(VendorSignals{ActiveMalware: true})
	if got != RiskHigh {
		t.Fatalf("got %q want high", got)
	}
}

func TestRiskExceeds(t *testing.T) {
	if !RiskExceeds(RiskMedium, RiskHigh) {
		t.Fatal("high should exceed medium")
	}
	if RiskExceeds(RiskHigh, RiskMedium) {
		t.Fatal("medium should not exceed high")
	}
}

func TestCrowdStrikeStatusMapping(t *testing.T) {
	cases := []struct {
		status    string
		contained bool
		healthy   bool
		risk      RiskLevel
	}{
		{"normal", false, true, RiskNone},
		{"Normal", false, true, RiskNone},
		{"containment_pending", false, false, RiskHigh},
		{"contained", true, false, RiskCritical},
		{"lift_containment_pending", false, false, RiskMedium},
		{"lost", false, false, RiskHigh},
	}
	for _, tc := range cases {
		if got := CrowdStrikeContainedFromStatus(tc.status); got != tc.contained {
			t.Fatalf("contained(%q) = %v want %v", tc.status, got, tc.contained)
		}
		if got := CrowdStrikeHealthyFromStatus(tc.status); got != tc.healthy {
			t.Fatalf("healthy(%q) = %v want %v", tc.status, got, tc.healthy)
		}
		if got := CrowdStrikeRiskFromStatus(tc.status); got != tc.risk {
			t.Fatalf("risk(%q) = %q want %q", tc.status, got, tc.risk)
		}
	}
}
