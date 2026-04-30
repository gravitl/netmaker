package extensions

import "github.com/gravitl/netmaker/orchestrator/extensions"

func NewProFactory() *extensions.Factory {
	return extensions.NewFactory(&ProNodeExtensions{}, &ProGatewayExtensions{})
}
