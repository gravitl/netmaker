package extensions

import "github.com/gravitl/netmaker/schema"

type GatewayExtensions interface {
	ConfigureAutoRelay(gateway *schema.Gateway)
}

type CEGatewayExtensions struct{}

func (c *CEGatewayExtensions) ConfigureAutoRelay(_ *schema.Gateway) {
	return
}
