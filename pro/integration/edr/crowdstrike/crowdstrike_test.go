package crowdstrike

import "testing"

func TestNormalizeDevice_ContainmentCritical(t *testing.T) {
	got := normalizeDevice(falconDevice{
		DeviceID:     "dev-1",
		SerialNumber: "SN1",
		Hostname:     "host-1",
		Status:       "containment",
	})
	if got.RiskLevel != "critical" || !got.Contained {
		t.Fatalf("expected contained critical, got risk=%q contained=%v", got.RiskLevel, got.Contained)
	}
}
