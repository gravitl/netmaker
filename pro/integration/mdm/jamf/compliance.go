package jamf

import (
	"strings"
)

// jamfDeviceTrustCompliant aggregates Jamf Conditional Access compliance records
// (GET /v1/conditional-access/device-compliance-information/{computer|mobile}/{id}).
// Every applicable record must be COMPLIANT; NON_COMPLIANT or UNKNOWN fails.
func jamfDeviceTrustCompliant(records []deviceComplianceInfo, filterVendors map[string]struct{}) bool {
	applicable := 0
	for _, r := range records {
		if !r.Applicable {
			continue
		}
		if len(filterVendors) > 0 {
			vendor := strings.ToLower(strings.TrimSpace(r.ComplianceVendor))
			if _, ok := filterVendors[vendor]; !ok {
				continue
			}
		}
		applicable++
		switch strings.ToUpper(strings.TrimSpace(r.ComplianceState)) {
		case "COMPLIANT":
			continue
		default:
			return false
		}
	}
	if len(filterVendors) > 0 && applicable == 0 {
		return false
	}
	return true
}

func complianceVendorSet(vendors []string) map[string]struct{} {
	if len(vendors) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(vendors))
	for _, v := range vendors {
		v = strings.ToLower(strings.TrimSpace(v))
		if v != "" {
			out[v] = struct{}{}
		}
	}
	return out
}

type deviceComplianceInfo struct {
	DeviceID        string `json:"deviceId"`
	Applicable      bool   `json:"applicable"`
	ComplianceState string `json:"complianceState"`
	ComplianceVendor string `json:"complianceVendor"`
}
