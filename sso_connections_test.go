// Copyright (c) 2026 Hyperping
// SPDX-License-Identifier: MIT

package hyperping

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Wire shapes as server/api/sso-connections/public.js sends them.
const oidcConnectionJSON = `{
  "uuid": "sso_abc123", "name": "Auth0", "provider": "oidc", "allowed_domains": ["acme.com"],
  "active": true, "configured": true, "issuer": "https://acme.eu.auth0.com/", "client_id": "client-1",
  "has_client_secret": true, "redirect_uri": "https://api.hyperping.io/v1/auth/sso/oidc/callback",
  "created_at": "2026-10-15T10:00:00.000Z", "updated_at": "2026-10-15T10:00:00.000Z"
}`

const samlConnectionJSON = `{
  "uuid": "sso_saml1", "name": "Okta", "provider": "okta", "allowed_domains": [], "active": true,
  "configured": true, "issuer": null, "client_id": null, "has_client_secret": false, "redirect_uri": null,
  "created_at": null, "updated_at": null
}`

type recordedRequest struct {
	method string
	path   string
	body   map[string]any
}

func ssoTestClient(t *testing.T, status int, body string, rec *recordedRequest) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rec != nil {
			rec.method, rec.path = r.Method, r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			if len(raw) > 0 {
				require.NoError(t, json.Unmarshal(raw, &rec.body))
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != "" {
			_, _ = w.Write([]byte(body))
		}
	}))
	t.Cleanup(server.Close)
	return NewClient("test-key", WithHTTPClient(server.Client()), WithBaseURL(server.URL), WithMaxRetries(0))
}

func TestClient_ListSSOConnections(t *testing.T) {
	var rec recordedRequest
	client := ssoTestClient(t, http.StatusOK, fmt.Sprintf(`{"connections": [%s, %s]}`, samlConnectionJSON, oidcConnectionJSON), &rec)

	connections, err := client.ListSSOConnections(context.Background())
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, rec.method)
	require.Equal(t, SSOConnectionsBasePath, rec.path)
	require.Len(t, connections, 2)

	saml := connections[0]
	require.Equal(t, "okta", saml.Provider)
	require.Nil(t, saml.Issuer)
	require.Nil(t, saml.RedirectURI)
	require.Equal(t, []string{}, saml.AllowedDomains)

	oidc := connections[1]
	require.Equal(t, SSOConnectionProviderOIDC, oidc.Provider)
	require.Equal(t, "https://acme.eu.auth0.com/", *oidc.Issuer)
	require.Equal(t, "client-1", *oidc.ClientID)
	require.True(t, oidc.HasClientSecret)
	require.Equal(t, "https://api.hyperping.io/v1/auth/sso/oidc/callback", *oidc.RedirectURI)
}

func TestClient_ListSSOConnections_Empty(t *testing.T) {
	client := ssoTestClient(t, http.StatusOK, `{"connections": null}`, nil)
	connections, err := client.ListSSOConnections(context.Background())
	require.NoError(t, err)
	require.NotNil(t, connections)
	require.Empty(t, connections)
}

func TestClient_GetSSOConnection(t *testing.T) {
	var rec recordedRequest
	client := ssoTestClient(t, http.StatusOK, fmt.Sprintf(`{"connection": %s}`, oidcConnectionJSON), &rec)

	connection, err := client.GetSSOConnection(context.Background(), "sso_abc123")
	require.NoError(t, err)
	require.Equal(t, SSOConnectionsBasePath+"/sso_abc123", rec.path)
	require.Equal(t, "Auth0", connection.Name)
	require.Equal(t, []string{"acme.com"}, connection.AllowedDomains)

	_, err = client.GetSSOConnection(context.Background(), "../statuspages")
	require.Error(t, err)

	notFound := ssoTestClient(t, http.StatusNotFound, `{"error": "SSO connection not found"}`, nil)
	_, err = notFound.GetSSOConnection(context.Background(), "sso_missing")
	require.Error(t, err)
	require.True(t, IsNotFound(err))
}

func TestClient_CreateSSOConnection(t *testing.T) {
	var rec recordedRequest
	client := ssoTestClient(t, http.StatusCreated, fmt.Sprintf(`{"message": "SSO connection created", "connection": %s}`, oidcConnectionJSON), &rec)

	connection, err := client.CreateSSOConnection(context.Background(), CreateSSOConnectionRequest{
		Name:           "Auth0",
		Issuer:         "https://acme.eu.auth0.com",
		ClientID:       "client-1",
		ClientSecret:   "secret-1",
		AllowedDomains: []string{"acme.com"},
	})
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, rec.method)
	require.Equal(t, SSOConnectionsBasePath, rec.path)
	require.Equal(t, map[string]any{
		"name": "Auth0", "issuer": "https://acme.eu.auth0.com", "client_id": "client-1",
		"client_secret": "secret-1", "allowed_domains": []any{"acme.com"},
	}, rec.body)
	require.Equal(t, "sso_abc123", connection.UUID)
	// The API answers with the issuer the identity provider publishes.
	require.Equal(t, "https://acme.eu.auth0.com/", *connection.Issuer)
}

func TestClient_CreateSSOConnection_OmitsUnsetDomains(t *testing.T) {
	var rec recordedRequest
	client := ssoTestClient(t, http.StatusCreated, fmt.Sprintf(`{"connection": %s}`, oidcConnectionJSON), &rec)
	_, err := client.CreateSSOConnection(context.Background(), CreateSSOConnectionRequest{Name: "Auth0", Issuer: "acme.eu.auth0.com", ClientID: "c", ClientSecret: "s"})
	require.NoError(t, err)
	// The API refuses allowed_domains: null; absent means "any domain".
	require.NotContains(t, rec.body, "allowed_domains")
	require.NotContains(t, rec.body, "provider")
}

func TestClient_CreateSSOConnection_RefusedByIdP(t *testing.T) {
	client := ssoTestClient(t, http.StatusBadRequest, `{"error": "The identity provider refused this client ID and secret (invalid_client).", "code": "client_refused"}`, nil)
	_, err := client.CreateSSOConnection(context.Background(), CreateSSOConnectionRequest{Name: "Auth0", Issuer: "acme.eu.auth0.com", ClientID: "c", ClientSecret: "wrong"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "refused this client ID and secret")
}

func TestClient_CreateSSOConnection_Validate(t *testing.T) {
	client := ssoTestClient(t, http.StatusCreated, `{}`, nil)
	_, err := client.CreateSSOConnection(context.Background(), CreateSSOConnectionRequest{Name: strings.Repeat("x", 256), Issuer: "i", ClientID: "c", ClientSecret: "s"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "name")
}

func TestClient_UpdateSSOConnection_SendsOnlySetFields(t *testing.T) {
	var rec recordedRequest
	client := ssoTestClient(t, http.StatusOK, fmt.Sprintf(`{"message": "SSO connection updated", "connection": %s}`, oidcConnectionJSON), &rec)

	empty := []string{}
	_, err := client.UpdateSSOConnection(context.Background(), "sso_abc123", UpdateSSOConnectionRequest{
		Name:           strPtr("Auth0 prod"),
		AllowedDomains: &empty,
	})
	require.NoError(t, err)
	require.Equal(t, http.MethodPut, rec.method)
	require.Equal(t, SSOConnectionsBasePath+"/sso_abc123", rec.path)
	// An empty list is sent (it clears the domains); unset fields are not.
	require.Equal(t, map[string]any{"name": "Auth0 prod", "allowed_domains": []any{}}, rec.body)
}

func TestClient_DeleteSSOConnection(t *testing.T) {
	var rec recordedRequest
	client := ssoTestClient(t, http.StatusOK, `{"message": "SSO connection deleted"}`, &rec)
	require.NoError(t, client.DeleteSSOConnection(context.Background(), "sso_abc123"))
	require.Equal(t, http.MethodDelete, rec.method)
	require.Equal(t, SSOConnectionsBasePath+"/sso_abc123", rec.path)

	inUse := ssoTestClient(t, http.StatusBadRequest, `{"error": "Cannot delete: 1 status page(s) are using this SSO connection. Remove SSO from those status pages first."}`, nil)
	err := inUse.DeleteSSOConnection(context.Background(), "sso_abc123")
	require.Error(t, err)
	require.Contains(t, err.Error(), "1 status page(s)")
}

func TestSSOConnectionRequests_StringRedactsSecret(t *testing.T) {
	create := CreateSSOConnectionRequest{Name: "Auth0", ClientID: "client-1", ClientSecret: "super-secret"}
	require.NotContains(t, create.String(), "super-secret")
	require.NotContains(t, fmt.Sprintf("%v", create), "super-secret")

	update := UpdateSSOConnectionRequest{ClientSecret: strPtr("super-secret")}
	require.NotContains(t, update.String(), "super-secret")
	require.NotContains(t, fmt.Sprintf("%+v", update), "super-secret")
	require.Contains(t, UpdateSSOConnectionRequest{}.String(), "ClientSecret:<nil>")
}
