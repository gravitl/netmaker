package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/gravitl/netmaker/models"
	"github.com/stretchr/testify/assert"
)

func TestParseNetworkStatusOptions(t *testing.T) {
	t.Run("defaults include everything", func(t *testing.T) {
		opts, err := parseNetworkStatusOptions(httptest.NewRequest("GET", "/api/v1/networks/net/status", nil))
		assert.NoError(t, err)
		assert.Empty(t, opts.Kinds)
		assert.True(t, opts.IncludePeers)
		assert.True(t, opts.IncludeEgress)
	})

	t.Run("filters", func(t *testing.T) {
		opts, err := parseNetworkStatusOptions(httptest.NewRequest("GET", "/api/v1/networks/net/status?src_type=user,%20extclient&dst_type=egress", nil))
		assert.NoError(t, err)
		assert.Len(t, opts.Kinds, 2)
		assert.Contains(t, opts.Kinds, models.NetworkNodeKindUser)
		assert.Contains(t, opts.Kinds, models.NetworkNodeKindExtClient)
		assert.False(t, opts.IncludePeers)
		assert.True(t, opts.IncludeEgress)
	})

	t.Run("invalid values", func(t *testing.T) {
		_, err := parseNetworkStatusOptions(httptest.NewRequest("GET", "/api/v1/networks/net/status?src_type=gateway", nil))
		assert.Error(t, err)
		_, err = parseNetworkStatusOptions(httptest.NewRequest("GET", "/api/v1/networks/net/status?dst_type=nodes", nil))
		assert.Error(t, err)
	})
}
