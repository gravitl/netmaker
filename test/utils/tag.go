package utils

import (
	"context"
	"testing"
	"time"

	"github.com/gravitl/netmaker/models"
	prologic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/require"
)

func CreateTag(t *testing.T, ctx context.Context, tagID, network string) *models.Tag {
	tag := models.Tag{
		ID:        models.TagID(tagID),
		TagName:   tagID,
		Network:   schema.NetworkID(network),
		CreatedAt: time.Now(),
	}
	err := prologic.UpsertTag(ctx, tag)
	require.NoError(t, err)

	return &tag
}

func DeleteTag(t *testing.T, ctx context.Context, tag *models.Tag) {
	err := (&schema.TagRecord{Key: tag.ID.String()}).Delete(ctx)
	require.NoError(t, err)
}
