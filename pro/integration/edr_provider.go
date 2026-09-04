package integration

import (
	"context"
	"encoding/json"

	edrpkg "github.com/gravitl/netmaker/pro/integration/edr"
)

type edrProvider struct {
	id ProviderID
}

func (e *edrProvider) Validate(configJSON json.RawMessage) error {
	return edrpkg.ValidateConfig(string(e.id), configJSON)
}

func (e *edrProvider) Test(configJSON json.RawMessage) error {
	if err := e.Validate(configJSON); err != nil {
		return err
	}
	p, err := edrpkg.Build(string(e.id), configJSON)
	if err != nil {
		return err
	}
	return p.Verify(context.Background())
}

func newEDRProvider(id ProviderID) Provider {
	return &edrProvider{id: id}
}
