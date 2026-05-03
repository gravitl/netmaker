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
}

func InitializeRepository(extFactory *extensions.Factory) {
	once.Do(func() {
		repo = &Repository{
			network: &NetworkOrchestrator{},
			node: &NodeOrchestrator{
				nodeExt: extFactory.NodeExtensions(),
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
