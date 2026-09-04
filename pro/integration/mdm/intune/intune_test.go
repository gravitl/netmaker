package intune

import "testing"

func TestManagedFromEntraDevice(t *testing.T) {
	got := managedFromEntraDevice(entraDevice{
		ID:          "ee155866-00b2-4476-9cba-c0dfa37f0224",
		DisplayName: "WIN-PMV0N6INPC6",
		IsManaged:   true,
		IsCompliant: true,
	}, "32f5f9ec-cd23-41e0-94e8-6b372232ff40")
	if !got.Enrolled || !got.Compliant {
		t.Fatalf("expected managed compliant entra device, got enrolled=%v compliant=%v",
			got.Enrolled, got.Compliant)
	}
	if got.AzureADDeviceID != "32f5f9ec-cd23-41e0-94e8-6b372232ff40" {
		t.Fatalf("unexpected azure ad device id: %q", got.AzureADDeviceID)
	}
	if got.ProviderDeviceID != "ee155866-00b2-4476-9cba-c0dfa37f0224" {
		t.Fatalf("unexpected provider device id: %q", got.ProviderDeviceID)
	}
}

func TestManagedFromManagedDeviceBackup(t *testing.T) {
	got := managedFromManagedDeviceBackup(managedDevice{
		ID:              "ee155866-00b2-4476-9cba-c0dfa37f0224",
		DeviceName:      "WIN-PMV0N6INPC6",
		AzureADDeviceID: "32f5f9ec-cd23-41e0-94e8-6b372232ff40",
		ComplianceState: "compliant",
		ManagementState: "managed",
	}, "32f5f9ec-cd23-41e0-94e8-6b372232ff40")
	if !got.Enrolled || !got.Compliant {
		t.Fatalf("expected enrolled compliant backup device, got enrolled=%v compliant=%v",
			got.Enrolled, got.Compliant)
	}
	if got.DeviceName != "WIN-PMV0N6INPC6" {
		t.Fatalf("unexpected device name: %q", got.DeviceName)
	}
	if got.ProviderDeviceID != "ee155866-00b2-4476-9cba-c0dfa37f0224" {
		t.Fatalf("unexpected provider device id: %q", got.ProviderDeviceID)
	}
}

func TestManagedFromManagedDeviceBackup_NotEnrolled(t *testing.T) {
	got := managedFromManagedDeviceBackup(managedDevice{
		DeviceName:      "WIN-PMV0N6INPC6",
		ComplianceState: "compliant",
		ManagementState: "discovered",
	}, "32f5f9ec-cd23-41e0-94e8-6b372232ff40")
	if got.Enrolled {
		t.Fatal("discovered device should not be enrolled")
	}
}

func TestIntuneComplianceCompliant(t *testing.T) {
	tests := []struct {
		state string
		want  bool
	}{
		{state: "compliant", want: true},
		{state: "Compliant", want: true},
		{state: "inGracePeriod", want: false},
		{state: "configManager", want: false},
		{state: "noncompliant", want: false},
		{state: "unknown", want: false},
		{state: "conflict", want: false},
	}
	for _, tc := range tests {
		if got := intuneComplianceCompliant(tc.state); got != tc.want {
			t.Fatalf("intuneComplianceCompliant(%q) = %v, want %v", tc.state, got, tc.want)
		}
	}
}

func TestIntuneDeviceEnrolled(t *testing.T) {
	tests := []struct {
		name string
		d    managedDevice
		want bool
	}{
		{
			name: "managed",
			d:    managedDevice{ManagementState: "managed"},
			want: true,
		},
		{
			name: "discovered",
			d:    managedDevice{ManagementState: "discovered"},
			want: false,
		},
		{
			name: "registered without management state",
			d:    managedDevice{DeviceRegistrationState: "registered"},
			want: true,
		},
		{
			name: "enrolled datetime fallback",
			d:    managedDevice{EnrolledDateTime: "2024-01-01T00:00:00Z"},
			want: true,
		},
		{
			name: "empty signals",
			d:    managedDevice{},
			want: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := intuneDeviceEnrolled(tc.d); got != tc.want {
				t.Fatalf("intuneDeviceEnrolled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNormalize_UsesIntuneAzureADDeviceID(t *testing.T) {
	d := managedDevice{
		DeviceName:      "laptop",
		AzureADDeviceID: "from-intune",
	}
	got := normalize(d)
	if got.AzureADDeviceID != "from-intune" {
		t.Fatalf("expected intune azureADDeviceId, got %q", got.AzureADDeviceID)
	}
}
