// Package auth builds a validator for Auth0-issued JWT access tokens.
package auth

import (
	"fmt"
	"net/url"

	"github.com/auth0/go-jwt-middleware/v3/jwks"
	"github.com/auth0/go-jwt-middleware/v3/validator"
)

// NewValidator builds a validator that verifies RS256 access tokens issued by
// the given Auth0 domain (e.g. "your-tenant.us.auth0.com") for the given
// audience (the API identifier configured in Auth0).
func NewValidator(domain, audience string) (*validator.Validator, error) {
	issuerURL, err := url.Parse(fmt.Sprintf("https://%s/", domain))
	if err != nil {
		return nil, fmt.Errorf("failed to parse issuer URL: %w", err)
	}

	provider, err := jwks.NewCachingProvider(jwks.WithIssuerURL(issuerURL))
	if err != nil {
		return nil, fmt.Errorf("failed to create JWKS provider: %w", err)
	}

	v, err := validator.New(
		validator.WithKeyFunc(provider.KeyFunc),
		validator.WithAlgorithm(validator.RS256),
		validator.WithIssuer(issuerURL.String()),
		validator.WithAudience(audience),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create validator: %w", err)
	}

	return v, nil
}
