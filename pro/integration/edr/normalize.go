package edr

import (
	"strings"
)

// RiskLevel is the vendor-agnostic endpoint risk classification.
type RiskLevel string

// Risk levels in ascending severity order.
const (
	RiskNone     = "none"
	RiskLow      = "low"
	RiskMedium   = "medium"
	RiskHigh     = "high"
	RiskCritical = "critical"
	RiskUnknown  = "unknown"
)

var riskOrder = map[RiskLevel]int{
	RiskNone:     0,
	RiskLow:      1,
	RiskMedium:   2,
	RiskHigh:     3,
	RiskCritical: 4,
}

// VendorSignals are provider-specific inputs normalized into RiskLevel.
type VendorSignals struct {
	AgentInstalled   bool
	AgentHealthy     bool
	Isolated         bool
	Contained        bool
	ActiveThreats    bool
	ActiveMalware    bool
	ActiveRansomware bool
	ThreatCount      int
	VendorRiskLevel  RiskLevel
}

// ComputeRiskLevel maps vendor signals to a vendor-agnostic risk level.
func ComputeRiskLevel(s VendorSignals) RiskLevel {
	if s.Contained || s.Isolated {
		return RiskCritical
	}
	if s.ActiveRansomware {
		return RiskCritical
	}

	threatLevel := RiskLevel(RiskNone)
	if s.ActiveMalware || s.ActiveThreats || s.ThreatCount > 0 {
		threatLevel = RiskHigh
	}

	vendorLevel := RiskLevel(RiskNone)
	if s.VendorRiskLevel != RiskNone && s.VendorRiskLevel != "" {
		vendorLevel = s.VendorRiskLevel
	}

	return maxRiskLevel(threatLevel, vendorLevel)
}

func maxRiskLevel(a, b RiskLevel) RiskLevel {
	if riskOrder[a] >= riskOrder[b] {
		return a
	}
	return b
}

// ParseRiskLevel normalizes a risk level string.
func ParseRiskLevel(level string) RiskLevel {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case RiskLow:
		return RiskLow
	case RiskMedium:
		return RiskMedium
	case RiskHigh:
		return RiskHigh
	case RiskCritical:
		return RiskCritical
	default:
		return RiskUnknown
	}
}

// RiskExceeds reports whether actual risk is strictly greater than max allowed.
func RiskExceeds(maxAllowed, actual RiskLevel) bool {
	return riskOrder[ParseRiskLevel(string(actual))] > riskOrder[ParseRiskLevel(string(maxAllowed))]
}

// DefenderRiskFromScore maps Microsoft Defender riskScore to normalized level.
func DefenderRiskFromScore(score string) RiskLevel {
	switch strings.ToLower(strings.TrimSpace(score)) {
	case "high":
		return RiskHigh
	case "medium":
		return RiskMedium
	case "low":
		return RiskLow
	case "none", "informational":
		return RiskNone
	default:
		return RiskNone
	}
}

// CrowdStrikeRiskFromStatus maps Falcon containment status to normalized level.
// Documented values: normal, containment_pending, contained, lift_containment_pending.
func CrowdStrikeRiskFromStatus(status string) RiskLevel {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "contained":
		return RiskCritical
	case "containment_pending":
		return RiskHigh
	case "lift_containment_pending":
		return RiskMedium
	case "lost":
		return RiskHigh
	case "normal":
		return RiskNone
	default:
		return RiskNone
	}
}

// CrowdStrikeContainedFromStatus reports whether Falcon status is contained.
func CrowdStrikeContainedFromStatus(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), "contained")
}

// CrowdStrikeHealthyFromStatus reports good operations per Falcon (status normal).
func CrowdStrikeHealthyFromStatus(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), "normal")
}

// SentinelOneRiskFromAgent maps SentinelOne agent fields to vendor risk hint.
func SentinelOneRiskFromAgent(infected bool, networkQuarantine bool, activeThreats int) RiskLevel {
	if networkQuarantine || infected {
		return RiskCritical
	}
	if activeThreats > 0 {
		return RiskHigh
	}
	return RiskNone
}

// WazuhRiskFromStatus maps Wazuh agent status to vendor risk hint.
func WazuhRiskFromStatus(status string) RiskLevel {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "disconnected":
		return RiskMedium
	case "never_connected", "pending":
		return RiskLow
	case "active":
		return RiskNone
	default:
		return RiskLow
	}
}

// WazuhHealthyFromStatus reports whether a Wazuh agent is connected and reporting.
func WazuhHealthyFromStatus(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), "active")
}
