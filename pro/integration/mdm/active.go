package mdm

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/gravitl/netmaker/schema"
)

const integrationType = "mdm"

// GetActive returns the configured MDM integration row, or nil if none exists.
func GetActive(ctx context.Context) (*schema.Integration, error) {
	intg := &schema.Integration{Type: integrationType}
	list, err := intg.ListByType(ctx)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	if len(list) > 1 {
		return nil, errors.New("multiple mdm integrations configured")
	}
	return &list[0], nil
}

// BuildActive builds the provider for the active MDM integration.
func BuildActive(ctx context.Context) (Provider, error) {
	intg, err := GetActive(ctx)
	if err != nil {
		return nil, err
	}
	if intg == nil {
		return nil, nil
	}
	return Build(intg.ID, json.RawMessage(intg.Config))
}

// ActiveProviderID returns the provider id of the active MDM integration, or "" if none.
func ActiveProviderID(ctx context.Context) (string, error) {
	intg, err := GetActive(ctx)
	if err != nil {
		return "", err
	}
	if intg == nil {
		return "", nil
	}
	return intg.ID, nil
}
