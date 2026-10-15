// Copyright (c) 2026 Hyperping
// SPDX-License-Identifier: MIT

package hyperping

import "fmt"

// =============================================================================
// Status Page SSO Connection Models
// =============================================================================

// SSOConnectionProviderOIDC is the provider of OpenID Connect connections, the
// only kind the API creates and changes. SAML (okta, azure, google, saml) and
// WorkOS connections are set up in the dashboard and are read-only here.
const SSOConnectionProviderOIDC = "oidc"

// SSOConnection is an identity provider that visitors of a private status page
// sign in with. A page uses it through
// StatusPageAuthenticationSettings.SSOConnectionUUID.
// API: GET /v2/sso-connections/{uuid}
type SSOConnection struct {
	UUID           string   `json:"uuid"` // e.g., "sso_abc123xyz"
	Name           string   `json:"name"`
	Provider       string   `json:"provider"` // oidc, okta, azure, google, saml, workos
	AllowedDomains []string `json:"allowed_domains"`
	Active         bool     `json:"active"`
	// Configured is true once the identity provider settings are saved.
	Configured bool `json:"configured"`
	// OpenID Connect only (nil for other providers). Issuer is the one the
	// identity provider publishes in its discovery document, which may differ
	// from the URL that was sent (trailing slash, discovery path).
	Issuer   *string `json:"issuer"`
	ClientID *string `json:"client_id"`
	// The client secret is write-only: the API only says whether one is saved.
	HasClientSecret bool `json:"has_client_secret"`
	// RedirectURI is the callback URL to register in the identity provider.
	RedirectURI *string `json:"redirect_uri"`
	CreatedAt   *string `json:"created_at"`
	UpdatedAt   *string `json:"updated_at"`
}

// CreateSSOConnectionRequest creates an OpenID Connect connection. The API
// checks the issuer's discovery document and the client credentials before
// saving anything.
// API: POST /v2/sso-connections
type CreateSSOConnectionRequest struct {
	Name     string `json:"name"`
	Provider string `json:"provider,omitempty"` // defaults to "oidc", the only accepted value
	// Issuer URL or domain of the identity provider, e.g. "https://tenant.eu.auth0.com".
	Issuer       string   `json:"issuer"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"` //nolint:gosec // G117: sent to the API, never returned or logged
	// AllowedDomains restricts sign-in to these email domains (empty = any).
	AllowedDomains []string `json:"allowed_domains,omitempty"`
}

// String never prints the client secret.
func (r CreateSSOConnectionRequest) String() string {
	return fmt.Sprintf("CreateSSOConnectionRequest{Name:%q Issuer:%q ClientID:%q ClientSecret:[redacted] AllowedDomains:%v}", r.Name, r.Issuer, r.ClientID, r.AllowedDomains)
}

// Validate checks input lengths on CreateSSOConnectionRequest fields.
func (r CreateSSOConnectionRequest) Validate() error {
	if err := validateStringLength("name", r.Name, maxNameLength); err != nil {
		return err
	}
	if err := validateStringLength("issuer", r.Issuer, maxURLLength); err != nil {
		return err
	}
	return validateStringLength("client_id", r.ClientID, maxNameLength)
}

// UpdateSSOConnectionRequest changes an OpenID Connect connection. Only the
// fields set are sent. Sending the identity provider settings already saved
// changes nothing; new ones are checked against the identity provider first,
// and a new ClientID needs its ClientSecret.
// API: PUT /v2/sso-connections/{uuid}
type UpdateSSOConnectionRequest struct {
	Name           *string   `json:"name,omitempty"`
	Issuer         *string   `json:"issuer,omitempty"`
	ClientID       *string   `json:"client_id,omitempty"`
	ClientSecret   *string   `json:"client_secret,omitempty"` //nolint:gosec // G117: sent to the API, never returned or logged
	AllowedDomains *[]string `json:"allowed_domains,omitempty"`
}

// String never prints the client secret.
func (r UpdateSSOConnectionRequest) String() string {
	secret := "<nil>"
	if r.ClientSecret != nil {
		secret = "[redacted]"
	}
	return fmt.Sprintf("UpdateSSOConnectionRequest{Name:%v Issuer:%v ClientID:%v ClientSecret:%s AllowedDomains:%v}",
		derefString(r.Name), derefString(r.Issuer), derefString(r.ClientID), secret, r.AllowedDomains)
}

// Validate checks input lengths on UpdateSSOConnectionRequest fields.
func (r UpdateSSOConnectionRequest) Validate() error {
	if r.Name != nil {
		if err := validateStringLength("name", *r.Name, maxNameLength); err != nil {
			return err
		}
	}
	if r.Issuer != nil {
		if err := validateStringLength("issuer", *r.Issuer, maxURLLength); err != nil {
			return err
		}
	}
	if r.ClientID != nil {
		return validateStringLength("client_id", *r.ClientID, maxNameLength)
	}
	return nil
}

func derefString(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%q", *s)
}
