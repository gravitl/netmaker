package utils

import (
	"testing"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/require"
)

func CreateTag(t *testing.T, tagID, network string) *models.Tag {
	tag := models.Tag{
		ID:        models.TagID(tagID),
		TagName:   tagID,
		Network:   schema.NetworkID(network),
		CreatedAt: time.Now(),
	}
	err := logic.UpsertTag(tag)
	require.NoError(t, err)

	return &tag
}

func DeleteTag(t *testing.T, tag *models.Tag) {
	err := database.DeleteRecord(database.TAG_TABLE_NAME, tag.ID.String())
	require.NoError(t, err)
}
