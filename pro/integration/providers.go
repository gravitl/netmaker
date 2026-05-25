package integration

import (
	"encoding/json"
	"fmt"
)

type Type string

type ProviderID string

const (
	TypeSIEM Type = "siem"
)

const (
	ProviderDatadog  ProviderID = "datadog"
	ProviderElastic  ProviderID = "elastic"
	ProviderSentinel ProviderID = "sentinel"
	ProviderSplunk   ProviderID = "splunk"
)

type Provider interface {
	Validate(config json.RawMessage) error
	Test(config json.RawMessage) error
}

var registry = map[Type]map[ProviderID]Provider{
	TypeSIEM: {
		ProviderSplunk:   &splunkProvider{},
		ProviderDatadog:  &datadogProvider{},
		ProviderElastic:  &elasticProvider{},
		ProviderSentinel: &sentinelProvider{},
	},
}

func Lookup(intType Type, id ProviderID) (Provider, error) {
	providers, ok := registry[intType]
	if !ok {
		return nil, fmt.Errorf("unknown integration type %q", intType)
	}
	p, ok := providers[id]
	if !ok {
		return nil, fmt.Errorf("unknown provider %q for type %q", id, intType)
	}
	return p, nil
}
