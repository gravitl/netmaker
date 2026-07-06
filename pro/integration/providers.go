package integration

import (
	"encoding/json"
	"fmt"

	"github.com/gravitl/netmaker/pro/integration/siem"
)

type Type string

type ProviderID string

const (
	TypeSIEM Type = "siem"
	TypeMDM  Type = "mdm"
	TypeEDR  Type = "edr"
)

const (
	ProviderDatadog  ProviderID = "datadog"
	ProviderElastic  ProviderID = "elastic"
	ProviderSentinel ProviderID = "sentinel"
	ProviderSplunk   ProviderID = "splunk"
	ProviderIntune    ProviderID = "intune"
	ProviderJamf      ProviderID = "jamf"
	ProviderJumpCloud ProviderID = "jumpcloud"
	ProviderIru       ProviderID = "iru"
	ProviderDefender    ProviderID = "defender"
	ProviderCrowdStrike ProviderID = "crowdstrike"
	ProviderSentinelOne ProviderID = "sentinelone"
	ProviderWazuh       ProviderID = "wazuh"
)

type Provider interface {
	Validate(config json.RawMessage) error
	Test(config json.RawMessage) error
}

var registry = map[Type]map[ProviderID]Provider{
	TypeSIEM: {
		ProviderDatadog:  siem.DatadogProvider(),
		ProviderElastic:  siem.ElasticProvider(),
		ProviderSentinel: siem.SentinelProvider(),
		ProviderSplunk:   siem.SplunkProvider(),
	},
	TypeMDM: {
		ProviderIntune:    newMDMProvider(ProviderIntune),
		ProviderJamf:      newMDMProvider(ProviderJamf),
		ProviderJumpCloud: newMDMProvider(ProviderJumpCloud),
		ProviderIru:       newMDMProvider(ProviderIru),
	},
	TypeEDR: {
		ProviderDefender:    newEDRProvider(ProviderDefender),
		ProviderCrowdStrike: newEDRProvider(ProviderCrowdStrike),
		ProviderSentinelOne: newEDRProvider(ProviderSentinelOne),
		ProviderWazuh:       newEDRProvider(ProviderWazuh),
	},
}

func Lookup(intType Type, id ProviderID) (Provider, error) {
	providers, ok := registry[intType]
	if !ok {
		return nil, fmt.Errorf("unknown integration type '%s'", intType)
	}
	p, ok := providers[id]
	if !ok {
		return nil, fmt.Errorf("unknown provider '%s' for type '%s'", id, intType)
	}
	return p, nil
}

// TypeExists reports whether the integration type is registered.
func TypeExists(intType Type) bool {
	_, ok := registry[intType]
	return ok
}
