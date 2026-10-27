//go:build ee

package logic

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func TestStatusFromSummary(t *testing.T) {
	live := func() schema.Node {
		return schema.Node{ID: uuid.NewString(), Connected: true, LastCheckIn: time.Now()}
	}
	stale := live()
	stale.LastCheckIn = time.Now().Add(-2 * models.LastCheckInThreshold)
	disconnected := live()
	disconnected.Connected = false

	peers := func(nodes ...schema.Node) (map[string]schema.Node, []string) {
		m := make(map[string]schema.Node, len(nodes))
		ids := make([]string, 0, len(nodes))
		for _, n := range nodes {
			m[n.ID] = n
			ids = append(ids, n.ID)
		}
		return m, ids
	}

	tests := []struct {
		name  string
		total int
		nodes []schema.Node
		extra []string // unreachable IDs with no node, e.g. deleted
		want  schema.NodeStatus
	}{
		{name: "all reachable", total: 4, want: schema.OnlineSt},
		{name: "minority unreachable", total: 4, nodes: []schema.Node{live()}, want: schema.OnlineSt},
		{name: "exactly half unreachable", total: 4, nodes: []schema.Node{live(), live()}, want: schema.OnlineSt},
		{name: "more than half unreachable", total: 4, nodes: []schema.Node{live(), live(), live()}, want: schema.WarningSt},
		{name: "none reachable", total: 3, nodes: []schema.Node{live(), live(), live()}, want: schema.ErrorSt},
		{name: "offline peers not counted", total: 3, nodes: []schema.Node{stale, disconnected, live()}, want: schema.ErrorSt},
		{name: "only offline peers unreachable", total: 3, nodes: []schema.Node{stale, disconnected}, want: schema.OnlineSt},
		{name: "deleted peers not counted", total: 3, nodes: []schema.Node{live()}, extra: []string{uuid.NewString(), uuid.NewString()}, want: schema.ErrorSt},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, ids := peers(tt.nodes...)
			summary := &models.MetricsSummary{TotalPeers: tt.total, UnreachablePeers: append(ids, tt.extra...)}
			if got := statusFromSummary(summary, m); got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}
