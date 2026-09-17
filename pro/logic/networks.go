package logic

import (
	"context"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func RemoveTagFromNetwork(ctx context.Context, tagID models.TagID, netID schema.NetworkID) error {
	network := &schema.Network{
		Name: string(netID),
	}
	err := network.Get(ctx)
	if err != nil {
		return err
	}

	tags := make(datatypes.JSONSlice[string], 0, len(network.AutoRemoveTags))

	for _, tag := range tags {
		if tag != tagID.String() {
			tags = append(tags, tag)
		}
	}

	network.AutoRemoveTags = tags
	return network.UpdateAutoRemoveTags(ctx)
}
