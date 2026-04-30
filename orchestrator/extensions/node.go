package extensions

import (
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

type NodeExtensions interface {
	ConfigureAutoAssignGateway(node *schema.Node, key *models.EnrollmentKey)
}

type CENodeExtensions struct{}

func (c *CENodeExtensions) ConfigureAutoAssignGateway(node *schema.Node, _ *models.EnrollmentKey) {
	node.AutoAssignGateway = false
}
