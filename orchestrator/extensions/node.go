package extensions

import (
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

type NodeExtensions interface {
	ConfigureAutoRelay(node *schema.Node)
	ConfigureAutoAssignGateway(node *schema.Node, key *models.EnrollmentKey)
	ConfigureTag(node *schema.Node, tagID models.TagID)
}

type CENodeExtensions struct{}

func (c *CENodeExtensions) ConfigureAutoRelay(_ *schema.Node) {}

func (c *CENodeExtensions) ConfigureAutoAssignGateway(node *schema.Node, _ *models.EnrollmentKey) {
	node.AutoAssignGateway = false
}

func (c *CENodeExtensions) ConfigureTag(_ *schema.Node, _ models.TagID) {}
