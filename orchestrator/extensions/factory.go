package extensions

type Factory struct {
	nodeExt NodeExtensions
	gwExt   GatewayExtensions
}

func NewFactory(nodeExt NodeExtensions, gwExt GatewayExtensions) *Factory {
	return &Factory{
		nodeExt: nodeExt,
		gwExt:   gwExt,
	}
}

func NewCEFactory() *Factory {
	return NewFactory(&CENodeExtensions{}, &CEGatewayExtensions{})
}

func (f *Factory) NodeExtensions() NodeExtensions {
	return f.nodeExt
}

func (f *Factory) GatewayExtensions() GatewayExtensions {
	return f.gwExt
}
