package jumpcloud

import "testing"

func TestDeviceTrustCompliant(t *testing.T) {
	ok := true
	no := false
	filter := map[string]struct{}{"p1": {}}

	tests := []struct {
		name    string
		results []policyResult
		filter  map[string]struct{}
		want    bool
	}{
		{name: "no results", results: nil, want: true},
		{name: "all success", results: []policyResult{
			{PolicyID: "p1", State: "success", Success: &ok},
		}, want: true},
		{name: "explicit failure", results: []policyResult{
			{PolicyID: "p1", State: "success", Success: &ok},
			{PolicyID: "p2", State: "failed", Success: &no},
		}, want: false},
		{name: "failed state", results: []policyResult{
			{PolicyID: "p1", State: "failed"},
		}, want: false},
		{name: "pending ignored", results: []policyResult{
			{PolicyID: "p1", State: "pending"},
		}, want: true},
		{name: "filter match pass", results: []policyResult{
			{PolicyID: "p1", State: "success", Success: &ok},
			{PolicyID: "p2", State: "failed", Success: &no},
		}, filter: filter, want: true},
		{name: "filter no match", results: []policyResult{
			{PolicyID: "p2", State: "success", Success: &ok},
		}, filter: filter, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := deviceTrustCompliant(tc.results, tc.filter)
			if got != tc.want {
				t.Fatalf("deviceTrustCompliant() = %v, want %v", got, tc.want)
			}
		})
	}
}
