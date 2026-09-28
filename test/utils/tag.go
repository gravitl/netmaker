package utils

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gravitl/netmaker/models"
	prologic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/require"
)

func CreateTag(t *testing.T, ctx context.Context, tagName, network string) *models.Tag {
	tagName = strings.TrimPrefix(tagName, network+".")
	tag := models.Tag{
		ID:        models.TagID(fmt.Sprintf("%s.%s", network, tagName)),
		TagName:   tagName,
		Network:   schema.NetworkID(network),
		CreatedAt: time.Now(),
	}
	err := prologic.UpsertTag(ctx, tag)
	require.NoError(t, err)

	return &tag
}

func DeleteTag(t *testing.T, ctx context.Context, tag *models.Tag) {
	network := &schema.Network{Name: tag.Network.String()}
	require.NoError(t, network.Get(ctx))

	err := (&schema.Tag{NetworkID: network.ID, Name: tag.TagName}).Delete(ctx)
	require.NoError(t, err)
}
