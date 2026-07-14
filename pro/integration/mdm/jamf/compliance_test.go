package jamf

import "testing"

func TestJamfDeviceTrustCompliant(t *testing.T) {
	filter := map[string]struct{}{"jamf": {}}

	tests := []struct {
		name    string
		records []deviceComplianceInfo
		filter  map[string]struct{}
		want    bool
	}{
		{name: "no records", want: true},
		{name: "not applicable ignored", records: []deviceComplianceInfo{
			{Applicable: false, ComplianceState: "NON_COMPLIANT"},
		}, want: true},
		{name: "compliant", records: []deviceComplianceInfo{
			{Applicable: true, ComplianceState: "COMPLIANT", ComplianceVendor: "Jamf"},
		}, want: true},
		{name: "non compliant", records: []deviceComplianceInfo{
			{Applicable: true, ComplianceState: "COMPLIANT"},
			{Applicable: true, ComplianceState: "NON_COMPLIANT"},
		}, want: false},
		{name: "unknown fails", records: []deviceComplianceInfo{
			{Applicable: true, ComplianceState: "UNKNOWN"},
		}, want: false},
		{name: "vendor filter pass", records: []deviceComplianceInfo{
			{Applicable: true, ComplianceState: "COMPLIANT", ComplianceVendor: "Jamf"},
			{Applicable: true, ComplianceState: "NON_COMPLIANT", ComplianceVendor: "Intune"},
		}, filter: filter, want: true},
		{name: "vendor filter missing", records: []deviceComplianceInfo{
			{Applicable: true, ComplianceState: "COMPLIANT", ComplianceVendor: "Intune"},
		}, filter: filter, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := jamfDeviceTrustCompliant(tc.records, tc.filter)
			if got != tc.want {
				t.Fatalf("jamfDeviceTrustCompliant() = %v, want %v", got, tc.want)
			}
		})
	}
}
