package iru

import "testing"

func TestDeviceCompliant(t *testing.T) {
	filter := map[string]struct{}{"lib-1": {}}

	tests := []struct {
		name   string
		status iruDeviceStatus
		filter map[string]struct{}
		want   bool
	}{
		{name: "empty status", status: iruDeviceStatus{}, want: true},
		{name: "all pass", status: iruDeviceStatus{
			Parameters:   []iruStatusItem{{Status: "PASS"}},
			LibraryItems: []iruStatusItem{{ItemID: "lib-1", Status: "PASS"}},
		}, want: true},
		{name: "parameter remediated", status: iruDeviceStatus{
			Parameters: []iruStatusItem{{Status: "REMEDIATED"}},
		}, want: true},
		{name: "library success", status: iruDeviceStatus{
			LibraryItems: []iruStatusItem{{ItemID: "lib-1", Status: "success"}},
		}, want: true},
		{name: "parameter error", status: iruDeviceStatus{
			Parameters: []iruStatusItem{{Status: "ERROR"}},
		}, want: false},
		{name: "parameter pending", status: iruDeviceStatus{
			Parameters: []iruStatusItem{{Status: "PENDING"}},
		}, want: false},
		{name: "library failed", status: iruDeviceStatus{
			LibraryItems: []iruStatusItem{{ItemID: "lib-1", Status: "failed"}},
		}, want: false},
		{name: "library pending", status: iruDeviceStatus{
			LibraryItems: []iruStatusItem{{ItemID: "lib-1", Status: "pending"}},
		}, want: false},
		{name: "filter match pass", status: iruDeviceStatus{
			LibraryItems: []iruStatusItem{
				{ItemID: "lib-1", Status: "PASS"},
				{ItemID: "lib-2", Status: "failed"},
			},
		}, filter: filter, want: true},
		{name: "filter no match", status: iruDeviceStatus{
			LibraryItems: []iruStatusItem{{ItemID: "lib-2", Status: "PASS"}},
		}, filter: filter, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := deviceCompliant(tc.status, tc.filter)
			if got != tc.want {
				t.Fatalf("deviceCompliant() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParameterStatusCompliant(t *testing.T) {
	pass := []string{"PASS", "pass", "REMEDIATED", "EXCLUDED", "WARNING"}
	for _, s := range pass {
		if !parameterStatusCompliant(s) {
			t.Fatalf("%q should be compliant", s)
		}
	}
	fail := []string{"ERROR", "INCOMPATIBLE", "PENDING", "failed", ""}
	for _, s := range fail {
		if parameterStatusCompliant(s) {
			t.Fatalf("%q should not be compliant", s)
		}
	}
}

func TestLibraryItemStatusCompliant(t *testing.T) {
	pass := []string{"PASS", "success", "SUCCESS", "EXCLUDED", "AVAILABLE"}
	for _, s := range pass {
		if !libraryItemStatusCompliant(s) {
			t.Fatalf("%q should be compliant", s)
		}
	}
	fail := []string{"ERROR", "INCOMPATIBLE", "PENDING", "pending", "failed", ""}
	for _, s := range fail {
		if libraryItemStatusCompliant(s) {
			t.Fatalf("%q should not be compliant", s)
		}
	}
}
