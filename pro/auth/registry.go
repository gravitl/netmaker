package auth

import (
	"context"
	"strings"
	"sync"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/gravitl/netmaker/servercfg"
)

var registryInitOnce sync.Once

type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[scope.Scope]map[string]Provider
}

var registry *ProviderRegistry

func Registry() *ProviderRegistry {
	registryInitOnce.Do(func() {
		registry = &ProviderRegistry{
			providers: make(map[scope.Scope]map[string]Provider),
		}
		registry.providers[scope.GlobalScope] = make(map[string]Provider)
		registry.providers[scope.OrgScope] = make(map[string]Provider)
		registry.providers[scope.TenantScope] = make(map[string]Provider)
	})
	return registry
}

func (p *ProviderRegistry) getOrInit(scopeLevel scope.Scope, scopeID string) (Provider, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	provider, ok := p.providers[scopeLevel][scopeID]
	if !ok {
		provider = p.initProvider(scopeLevel, scopeID)
		if provider == nil {
			return nil, false
		}

		p.providers[scopeLevel][scopeID] = provider
	}

	return provider, true
}

func (p *ProviderRegistry) Delete(scopeLevel scope.Scope, scopeID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.providers[scopeLevel], scopeID)
}

// FromContext returns the provider for the scope embedded in ctx.
func (p *ProviderRegistry) FromContext(ctx context.Context) (Provider, bool) {
	return p.getOrInit(scope.Level(ctx), scope.ID(ctx))
}

func (p *ProviderRegistry) initProvider(scopeLevel scope.Scope, scopeID string) Provider {
	redirectUrl := p.getRedirectURL()
	switch scopeLevel {
	case scope.OrgScope:
		settingsRecord := &schema.OrganizationSettings{
			ID: scopeID,
		}
		err := settingsRecord.Get(db.WithContext(context.TODO()))
		if err != nil {
			return nil
		}

		settings := settingsRecord.Settings.Data()
		switch settings.AuthProvider {
		case google_provider_name:
			return NewGoogleProvider(redirectUrl, settings.ClientID, settings.ClientSecret)
		case azure_ad_provider_name:
			return NewAzureADProvider(redirectUrl, settings.ClientID, settings.ClientSecret, settings.AzureTenant)
		case github_provider_name:
			return NewGitHubProvider(redirectUrl, settings.ClientID, settings.ClientSecret)
		case okta_provider_name:
			provider, err := NewOktaProvider(redirectUrl, settings.ClientID, settings.ClientSecret, settings.OIDCIssuer)
			if err != nil {
				return nil
			}
			return provider
		case oidc_provider_name:
			provider, err := NewOIDCProvider(redirectUrl, settings.ClientID, settings.ClientSecret, settings.OIDCIssuer)
			if err != nil {
				return nil
			}
			return provider
		}

		return nil
	case scope.TenantScope:
		settingsRecord := &schema.TenantSettingsRecord{
			Key: scopeID,
		}
		err := settingsRecord.Get(db.WithContext(context.TODO()))
		if err != nil {
			return nil
		}

		settings := settingsRecord.Value.Data()
		switch settings.AuthProvider {
		case google_provider_name:
			return NewGoogleProvider(redirectUrl, settings.ClientID, settings.ClientSecret)
		case azure_ad_provider_name:
			return NewAzureADProvider(redirectUrl, settings.ClientID, settings.ClientSecret, settings.AzureTenant)
		case github_provider_name:
			return NewGitHubProvider(redirectUrl, settings.ClientID, settings.ClientSecret)
		case okta_provider_name:
			provider, err := NewOktaProvider(redirectUrl, settings.ClientID, settings.ClientSecret, settings.OIDCIssuer)
			if err != nil {
				return nil
			}
			return provider
		case oidc_provider_name:
			provider, err := NewOIDCProvider(redirectUrl, settings.ClientID, settings.ClientSecret, settings.OIDCIssuer)
			if err != nil {
				return nil
			}
			return provider
		}

		return nil
	default:
		return nil
	}
}

func (p *ProviderRegistry) getRedirectURL() string {
	apiHost := servercfg.GetAPIHost()
	if strings.Contains(apiHost, "localhost") || strings.Contains(apiHost, "127.0.0.1") {
		apiHost = "http://" + apiHost
	} else {
		apiHost = "https://" + apiHost
	}

	return apiHost + "/api/oauth/callback"
}
