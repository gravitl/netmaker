package logic

import (
	"testing"
	"time"

	"github.com/gravitl/netmaker/models"
)

func TestIngressGatewayIsReachable(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		gw   *models.Node
		want bool
	}{
		{name: "nil", want: false},
		{
			name: "online",
			gw: &models.Node{
				CommonNode:  models.CommonNode{Connected: true},
				LastCheckIn: now,
			},
			want: true,
		},
		{
			name: "disconnected",
			gw: &models.Node{
				CommonNode:  models.CommonNode{Connected: false},
				LastCheckIn: now,
			},
			want: false,
		},
		{
			name: "stale check-in",
			gw: &models.Node{
				CommonNode:  models.CommonNode{Connected: true},
				LastCheckIn: now.Add(-2 * models.LastCheckInThreshold),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IngressGatewayIsReachable(tt.gw); got != tt.want {
				t.Fatalf("IngressGatewayIsReachable = %v, want %v", got, tt.want)
			}
		})
	}
}
