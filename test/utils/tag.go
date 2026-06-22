package utils

import (
	"context"
	"testing"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/require"
)

func CreateTag(t *testing.T, tagID, network string) *schema.Tag {
	tag := schema.Tag{
		ID:        schema.TagID(tagID),
		TagName:   tagID,
		Network:   schema.NetworkID(network),
		CreatedAt: time.Now(),
	}
	err := logic.UpsertTag(tag)
	require.NoError(t, err)

	return &tag
}

func DeleteTag(t *testing.T, tag *schema.Tag) {
	err := (&schema.TagEntry{Key: tag.ID.String()}).Delete(db.WithContext(context.TODO()))
	require.NoError(t, err)
}
