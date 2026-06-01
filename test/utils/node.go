package utils

import (
	"context"
	"testing"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/require"
)

func DeleteNode(t *testing.T, node *schema.Node) {
	err := node.Delete(db.WithContext(context.TODO()))
	require.NoError(t, err)
}
