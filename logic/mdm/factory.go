package mdm

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

// GetProviderFromIntegration takes a schema.Integration (from the DB) and returns an initialized MDMProvider.
func GetProviderFromIntegration(integration schema.Integration) (models.MDMProvider, error) {
	if integration.Type != "mdm" {
		return nil, fmt.Errorf("integration is not of type mdm (got %s)", integration.Type)
	}

	var config models.MDMConfig
	if err := json.Unmarshal(integration.Config, &config); err != nil {
		return nil, fmt.Errorf("failed to parse MDM config: %w", err)
	}

	var provider models.MDMProvider

	// Select the implementation based on the provider string
	switch models.MDMProviderType(integration.Provider) {
	case models.MDMProviderIntune:
		provider = &IntuneProvider{}
	case models.MDMProviderJamf:
		// TODO: Implement JamfProvider
		return nil, errors.New("Jamf provider is not yet implemented")
	case models.MDMProviderKandji:
		// TODO: Implement KandjiProvider
		return nil, errors.New("Kandji provider is not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported MDM provider type: %s", integration.Provider)
	}

	// Initialize the provider with the parsed config (fetches initial OAuth tokens, etc.)
	if err := provider.Initialize(config); err != nil {
		return nil, fmt.Errorf("failed to initialize %s provider: %w", integration.Provider, err)
	}

	return provider, nil
}
