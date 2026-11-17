package logic

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
)

func TestResetAutoRelayedPeer_EmptyNoOp(t *testing.T) {
	node := &models.Node{AutoRelayedPeers: nil}
	node.ID = uuid.New()
	if err := ResetAutoRelayedPeer(context.Background(), node); err != nil {
		t.Fatalf("expected nil error for empty peers, got %v", err)
	}

	node.AutoRelayedPeers = map[string]string{}
	if err := ResetAutoRelayedPeer(context.Background(), node); err != nil {
		t.Fatalf("expected nil error for empty map, got %v", err)
	}
}

func TestClearAutoRelayedPeerPair_NilArgs(t *testing.T) {
	node := &models.Node{}
	node.ID = uuid.New()
	if err := ClearAutoRelayedPeerPair(nil, node); err == nil {
		t.Fatal("expected error for nil node")
	}
	if err := ClearAutoRelayedPeerPair(node, nil); err == nil {
		t.Fatal("expected error for nil peer")
	}
}

func TestRemoveAutoRelayedPeerPairFromMaps(t *testing.T) {
	relayID := uuid.New().String()
	nodeID := uuid.New()
	peerID := uuid.New()
	otherPeerID := uuid.New().String()

	node := &models.Node{
		AutoRelayedPeers: map[string]string{
			peerID.String(): peerID.String(),
			otherPeerID:     relayID,
		},
	}
	node.ID = nodeID
	peer := &models.Node{
		AutoRelayedPeers: map[string]string{
			nodeID.String(): relayID,
		},
	}
	peer.ID = peerID

	removeAutoRelayedPeerPairFromMaps(node, peer)

	if _, exists := node.AutoRelayedPeers[peerID.String()]; exists {
		t.Fatal("expected peer entry removed from node")
	}
	if _, exists := peer.AutoRelayedPeers[nodeID.String()]; exists {
		t.Fatal("expected node entry removed from peer")
	}
	if got, ok := node.AutoRelayedPeers[otherPeerID]; !ok || got != relayID {
		t.Fatalf("expected unrelated peer entry preserved, got ok=%v val=%q", ok, got)
	}

	// nil maps and nil args are safe no-ops
	removeAutoRelayedPeerPairFromMaps(nil, peer)
	removeAutoRelayedPeerPairFromMaps(node, nil)
	nilNode := &models.Node{AutoRelayedPeers: nil}
	nilNode.ID = nodeID
	nilPeer := &models.Node{AutoRelayedPeers: nil}
	nilPeer.ID = peerID
	removeAutoRelayedPeerPairFromMaps(nilNode, nilPeer)
}
