package extensions

type Factory struct {
	userExt UserExtensions
	nodeExt NodeExtensions
}

func NewFactory(userExt UserExtensions, nodeExt NodeExtensions) *Factory {
	return &Factory{
		userExt: userExt,
		nodeExt: nodeExt,
	}
}

func NewCEFactory() *Factory {
	return NewFactory(&CEUserExtensions{}, &CENodeExtensions{})
}

func (f *Factory) UserExtensions() UserExtensions {
	return f.userExt
}

func (f *Factory) NodeExtensions() NodeExtensions {
	return f.nodeExt
}
