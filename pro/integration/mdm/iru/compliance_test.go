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
		{name: "parameter fail", status: iruDeviceStatus{
			Parameters: []iruStatusItem{{Status: "FAIL"}},
		}, want: false},
		{name: "library item fail", status: iruDeviceStatus{
			LibraryItems: []iruStatusItem{{ItemID: "lib-1", Status: "FAIL"}},
		}, want: false},
		{name: "filter match pass", status: iruDeviceStatus{
			LibraryItems: []iruStatusItem{
				{ItemID: "lib-1", Status: "PASS"},
				{ItemID: "lib-2", Status: "FAIL"},
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

func TestStatusItemFailed(t *testing.T) {
	if statusItemFailed("PASS") {
		t.Fatal("PASS should not fail")
	}
	if statusItemFailed("pass") {
		t.Fatal("pass should not fail")
	}
	if !statusItemFailed("FAIL") {
		t.Fatal("FAIL should fail")
	}
	if !statusItemFailed("") {
		t.Fatal("empty status should fail")
	}
}
