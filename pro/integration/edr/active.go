package edr

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/gravitl/netmaker/schema"
)

const integrationType = "edr"

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
		return nil, errors.New("multiple edr integrations configured")
	}
	return &list[0], nil
}

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
