package orchestrator

import (
	"sync"

	"github.com/gravitl/netmaker/orchestrator/extensions"
)

var repo *Repository
var once sync.Once

type Repository struct {
	network *NetworkOrchestrator
	node    *NodeOrchestrator
	gateway *GatewayOrchestrator
}

func InitializeRepository(extFactory *extensions.Factory) {
	once.Do(func() {
		repo = &Repository{
			network: &NetworkOrchestrator{},
			node: &NodeOrchestrator{
				nodeExt: extFactory.NodeExtensions(),
			},
			gateway: &GatewayOrchestrator{
				gwExt: extFactory.GatewayExtensions(),
			},
		}
	})
}

func GetRepository() *Repository {
	return repo
}

func (r *Repository) NetworkOrchestrator() *NetworkOrchestrator {
	return r.network
}

func (r *Repository) NodeOrchestrator() *NodeOrchestrator {
	return r.node
}

func (r *Repository) GatewayOrchestrator() *GatewayOrchestrator {
	return r.gateway
}
