package extensions

import (
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

type ProNodeExtensions struct{}

func (p *ProNodeExtensions) ConfigureAutoRelay(node *schema.Node) {
	node.IsAutoRelay = true
}

func (p *ProNodeExtensions) ConfigureAutoAssignGateway(node *schema.Node, key *models.EnrollmentKey) {
	node.AutoAssignGateway = key.AutoAssignGateway
}
