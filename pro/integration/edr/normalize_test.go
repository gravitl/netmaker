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
