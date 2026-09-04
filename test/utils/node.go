package utils

import (
	"context"
	"testing"

	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/require"
)

func DeleteNode(t *testing.T, ctx context.Context, node *schema.Node) {
	err := node.Delete(ctx)
	require.NoError(t, err)
}
