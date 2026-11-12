package extensions

import (
	"context"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

type NodeExtensions interface {
	ConfigureAutoAssignGateway(node *schema.Node, key *schema.EnrollmentKey)
	ConfigureTag(ctx context.Context, node *schema.Node, tagID models.TagID)
}

type CENodeExtensions struct{}

func (c *CENodeExtensions) ConfigureAutoAssignGateway(node *schema.Node, _ *schema.EnrollmentKey) {
	node.AutoAssignGateway = false
}

func (c *CENodeExtensions) ConfigureTag(_ context.Context, _ *schema.Node, _ models.TagID) {}
