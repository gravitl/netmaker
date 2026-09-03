package migrate

import (
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
)

func TestCollectLegacyIGWClients(t *testing.T) {
	gwID := uuid.New()
	clientA := uuid.New()
	clientB := uuid.New()
	otherGw := uuid.New()

	gw := models.Node{
		CommonNode: models.CommonNode{ID: gwID, Network: "net1"},
		InetNodeReq: models.InetNodeReq{
			InetNodeClientIDs: []string{clientA.String()},
		},
	}
	all := []models.Node{
		gw,
		{CommonNode: models.CommonNode{ID: clientA, Network: "net1"}, InternetGwID: gwID.String()},
		{CommonNode: models.CommonNode{ID: clientB, Network: "net1"}, InternetGwID: gwID.String()},
		{CommonNode: models.CommonNode{ID: uuid.New(), Network: "net1"}, InternetGwID: otherGw.String()},
		{CommonNode: models.CommonNode{ID: uuid.New(), Network: "net2"}, InternetGwID: gwID.String()},
	}

	got := collectLegacyIGWClients(gw, all)
	if len(got) != 2 {
		t.Fatalf("expected 2 clients, got %d: %v", len(got), got)
	}
	seen := map[string]struct{}{}
	for _, id := range got {
		seen[id] = struct{}{}
	}
	if _, ok := seen[clientA.String()]; !ok {
		t.Fatalf("missing clientA in %v", got)
	}
	if _, ok := seen[clientB.String()]; !ok {
		t.Fatalf("missing clientB in %v", got)
	}
}

func TestAllExtClientsMissingInternetEgressSelection(t *testing.T) {
	if !allExtClientsMissingInternetEgressSelection([]models.ExtClient{
		{ClientID: "a"},
		{ClientID: "b"},
	}) {
		t.Fatal("expected true when all selections empty")
	}
	if allExtClientsMissingInternetEgressSelection([]models.ExtClient{
		{ClientID: "a"},
		{ClientID: "b", SelectedInternetEgressID: "eg-1"},
	}) {
		t.Fatal("expected false when any selection is set")
	}
	if !allExtClientsMissingInternetEgressSelection(nil) {
		t.Fatal("expected true for empty list")
	}
}
