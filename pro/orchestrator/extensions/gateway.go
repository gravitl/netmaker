package extensions

import "github.com/gravitl/netmaker/schema"

type ProGatewayExtensions struct{}

func (p *ProGatewayExtensions) ConfigureAutoRelay(gateway *schema.Gateway) {
	gateway.IsAutoRelay = true
}
