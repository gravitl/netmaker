package controller

import (
	"testing"

	"github.com/gravitl/netmaker/schema"
)

func TestShouldApplyEgressDomainAnsUpdate(t *testing.T) {
	tests := []struct {
		name     string
		ranges   []string
		expected bool
	}{
		{
			name:     "applies when ranges exist",
			ranges:   []string{"10.0.0.0/24"},
			expected: true,
		},
		{
			name:     "skips when no ranges",
			ranges:   nil,
			expected: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := shouldApplyEgressDomainAnsUpdate(schema.Egress{}, tc.ranges)
			if got != tc.expected {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}
