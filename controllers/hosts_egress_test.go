package controller

import (
	"testing"

	"github.com/gravitl/netmaker/schema"
)

func TestShouldApplyEgressDomainAnsUpdate(t *testing.T) {
	tests := []struct {
		name     string
		e        schema.Egress
		ranges   []string
		expected bool
	}{
		{
			name:     "applies when ranges exist and not static",
			e:        schema.Egress{StaticDomainAns: false},
			ranges:   []string{"10.0.0.0/24"},
			expected: true,
		},
		{
			name:     "skips when static domain ans",
			e:        schema.Egress{StaticDomainAns: true},
			ranges:   []string{"10.0.0.0/24"},
			expected: false,
		},
		{
			name:     "skips when no ranges",
			e:        schema.Egress{StaticDomainAns: false},
			ranges:   nil,
			expected: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := shouldApplyEgressDomainAnsUpdate(tc.e, tc.ranges)
			if got != tc.expected {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}
