package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/gravitl/netmaker/models"
)

func validServerSettings() models.ServerSettings {
	return models.ServerSettings{
		JwtValidityDuration:        60,
		JwtValidityDurationClients: 60,
		IPDetectionInterval:        15,
		Verbosity:                  1,
		MetricsPort:                8889,
		CleanUpInterval:            10,
		MetricInterval:             "15",
	}
}

func TestValidateNewSettingsMetricInterval(t *testing.T) {
	tests := []struct {
		name     string
		interval string
		wantErr  error
	}{
		{name: "valid", interval: "15", wantErr: nil},
		{name: "valid with spaces", interval: " 5 ", wantErr: nil},
		{name: "empty uses fallback", interval: "", wantErr: nil},
		{name: "zero", interval: "0", wantErr: ErrInvalidMetricInterval},
		{name: "negative", interval: "-5", wantErr: ErrInvalidMetricInterval},
		{name: "not a number", interval: "abc", wantErr: ErrInvalidMetricInterval},
		{name: "decimal", interval: "1.5", wantErr: ErrInvalidMetricInterval},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validServerSettings()
			req.MetricInterval = tt.interval
			err := ValidateNewSettings(context.Background(), req)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateNewSettings(%q) = %v, want %v", tt.interval, err, tt.wantErr)
			}
		})
	}
}
