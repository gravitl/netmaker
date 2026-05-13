package extensions

import (
	"github.com/gravitl/netmaker/models"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/schema"
)

type ProNodeExtensions struct{}

func (p *ProNodeExtensions) ConfigureAutoRelay(node *schema.Node) {
	node.IsAutoRelay = true
}

func (p *ProNodeExtensions) ConfigureAutoAssignGateway(node *schema.Node, key *models.EnrollmentKey) {
	node.AutoAssignGateway = key.AutoAssignGateway
}

func (p *ProNodeExtensions) ConfigureTag(node *schema.Node, tagID models.TagID) {
	tag, err := proLogic.GetTag(tagID)
	if err != nil {
		return
	}

	if tag.Network.String() == node.Network.Name {
		node.Tags[string(tagID)] = true
	}
}
