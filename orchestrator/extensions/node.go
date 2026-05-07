package extensions

import (
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

type NodeExtensions interface {
	ConfigureAutoRelay(node *schema.Node)
	ConfigureAutoAssignGateway(node *schema.Node, key *models.EnrollmentKey)
}

type CENodeExtensions struct{}

func (c *CENodeExtensions) ConfigureAutoRelay(_ *schema.Node) {
}

func (c *CENodeExtensions) ConfigureAutoAssignGateway(node *schema.Node, _ *models.EnrollmentKey) {
	node.AutoAssignGateway = false
}
