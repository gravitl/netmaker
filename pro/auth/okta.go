package auth

// NewOktaProvider constructs an OIDCProvider configured for Okta.
// Okta uses standard OIDC discovery; this is a thin alias over NewOIDCProvider.
func NewOktaProvider(redirectURL, clientID, clientSecret, issuer string) (*OIDCProvider, error) {
	p, err := NewOIDCProvider(redirectURL, clientID, clientSecret, issuer)
	if err != nil {
		return nil, err
	}
	p.name = okta_provider_name
	return p, nil
}
