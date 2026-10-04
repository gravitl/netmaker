package logic

import (
	"context"
	"testing"

	"github.com/gravitl/netmaker/models"
)

func TestFailOpenExitClientsKeepSelectionEmpty(t *testing.T) {
	got := FailOpenExitClientsKeepSelection(context.Background(), nil)
	if got != nil && len(got) != 0 {
		t.Fatalf("nil/empty input: got %#v", got)
	}
	got = FailOpenExitClientsKeepSelection(context.Background(), []models.Node{})
	if len(got) != 0 {
		t.Fatalf("empty slice: got %d", len(got))
	}
}
