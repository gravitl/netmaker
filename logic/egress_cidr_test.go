package logic

import (
	"testing"

	"github.com/gravitl/netmaker/schema"
)

func TestValidateEgressCIDR(t *testing.T) {
	network := &schema.Network{
		Name:         "test-net",
		AddressRange: "10.100.0.0/16",
	}

	tests := []struct {
		name    string
		cidr    string
		wantErr bool
	}{
		{"empty ok", "", false},
		{"inet gw ok", "*", false},
		{"disjoint ok", "192.168.0.0/16", false},
		{"overlap network", "10.100.0.0/24", true},
		{"contains network", "10.0.0.0/8", true},
		{"loopback host", "127.0.0.1/32", true},
		{"loopback block", "127.0.0.0/8", true},
		{"ipv6 loopback", "::1/128", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateEgressCIDR(network, tc.cidr)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateEgressCIDR(%q) err=%v wantErr=%v", tc.cidr, err, tc.wantErr)
			}
		})
	}
}
