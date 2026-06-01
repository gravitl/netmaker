package extensions

type Factory struct {
	nodeExt NodeExtensions
}

func NewFactory(nodeExt NodeExtensions) *Factory {
	return &Factory{
		nodeExt: nodeExt,
	}
}

func NewCEFactory() *Factory {
	return NewFactory(&CENodeExtensions{})
}

func (f *Factory) NodeExtensions() NodeExtensions {
	return f.nodeExt
}
