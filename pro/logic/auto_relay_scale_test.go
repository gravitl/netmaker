package logic

import (
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/stretchr/testify/assert"
)

func TestAutoRelayPairAlreadySet(t *testing.T) {
	a := uuid.New()
	b := uuid.New()
	gw := uuid.New().String()
	victim := models.Node{CommonNode: models.CommonNode{ID: a}}
	peer := models.Node{CommonNode: models.CommonNode{ID: b}}
	assert.False(t, AutoRelayPairAlreadySet(victim, peer, gw))

	victim.AutoRelayedPeers = map[string]string{b.String(): gw}
	peer.AutoRelayedPeers = map[string]string{a.String(): gw}
	assert.True(t, AutoRelayPairAlreadySet(victim, peer, gw))
	assert.False(t, AutoRelayPairAlreadySet(victim, peer, uuid.New().String()))
}

func TestIsAutoRelayPairWriter(t *testing.T) {
	low := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	high := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	assert.True(t, IsAutoRelayPairWriter(low, high))
	assert.False(t, IsAutoRelayPairWriter(high, low))
	assert.True(t, IsAutoRelayPairWriter(low, low))
}
