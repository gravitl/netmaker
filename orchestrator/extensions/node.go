package extensions

import (
	"github.com/gravitl/netmaker/schema"
)

type NodeExtensions interface {
	ConfigureAutoRelay(node *schema.Node)
	ConfigureAutoAssignGateway(node *schema.Node, key *schema.EnrollmentKey)
	ConfigureTag(node *schema.Node, tagID schema.TagID)
}

type CENodeExtensions struct{}

func (c *CENodeExtensions) ConfigureAutoRelay(node *schema.Node) {
	node.IsAutoRelay = "no"
}

func (c *CENodeExtensions) ConfigureAutoAssignGateway(node *schema.Node, _ *schema.EnrollmentKey) {
	node.AutoAssignGateway = false
}

func (c *CENodeExtensions) ConfigureTag(_ *schema.Node, _ schema.TagID) {}
