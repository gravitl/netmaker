package integration

import (
	"context"
	"encoding/json"

	mdmpkg "github.com/gravitl/netmaker/pro/integration/mdm"
)

type mdmProvider struct {
	id ProviderID
}

func (m *mdmProvider) Validate(configJSON json.RawMessage) error {
	return mdmpkg.ValidateConfig(string(m.id), configJSON)
}

func (m *mdmProvider) Test(configJSON json.RawMessage) error {
	if err := m.Validate(configJSON); err != nil {
		return err
	}
	p, err := mdmpkg.Build(string(m.id), configJSON)
	if err != nil {
		return err
	}
	return p.Verify(context.Background())
}

func newMDMProvider(id ProviderID) Provider {
	return &mdmProvider{id: id}
}
