// Copyright (c) 2026 Hyperping
// SPDX-License-Identifier: MIT

package hyperping

import (
	"context"
	"fmt"
	"net/http"
)

// ListSSOConnections returns every status-page SSO connection of the API key's
// project (OpenID Connect, SAML and WorkOS).
func (c *Client) ListSSOConnections(ctx context.Context) ([]SSOConnection, error) {
	// API returns: {"connections": [...]}
	var response struct {
		Connections []SSOConnection `json:"connections"`
	}
	if err := c.doRequest(ctx, http.MethodGet, SSOConnectionsBasePath, nil, &response); err != nil {
		return nil, fmt.Errorf("failed to list SSO connections: %w", err)
	}
	if response.Connections == nil {
		response.Connections = []SSOConnection{}
	}
	return response.Connections, nil
}

// GetSSOConnection returns a single SSO connection by UUID.
func (c *Client) GetSSOConnection(ctx context.Context, uuid string) (*SSOConnection, error) {
	if err := ValidateResourceID(uuid); err != nil {
		return nil, fmt.Errorf("GetSSOConnection: %w", err)
	}

	// API returns: {"connection": {...}}
	var response struct {
		Connection SSOConnection `json:"connection"`
	}
	path := fmt.Sprintf("%s/%s", SSOConnectionsBasePath, uuid)
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, fmt.Errorf("failed to get SSO connection %s: %w", uuid, err)
	}
	return &response.Connection, nil
}

// CreateSSOConnection creates an OpenID Connect connection. It is billed like
// one created in the dashboard.
func (c *Client) CreateSSOConnection(ctx context.Context, req CreateSSOConnectionRequest) (*SSOConnection, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("CreateSSOConnection: %w", err)
	}

	// API returns: {"message": "SSO connection created", "connection": {...}}
	var response struct {
		Message    string        `json:"message"`
		Connection SSOConnection `json:"connection"`
	}
	if err := c.doRequest(ctx, http.MethodPost, SSOConnectionsBasePath, req, &response); err != nil {
		return nil, fmt.Errorf("failed to create SSO connection: %w", err)
	}
	return &response.Connection, nil
}

// UpdateSSOConnection changes an OpenID Connect connection. Only the fields
// set in req are sent.
func (c *Client) UpdateSSOConnection(ctx context.Context, uuid string, req UpdateSSOConnectionRequest) (*SSOConnection, error) {
	if err := ValidateResourceID(uuid); err != nil {
		return nil, fmt.Errorf("UpdateSSOConnection: %w", err)
	}
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("UpdateSSOConnection: %w", err)
	}

	// API returns: {"message": "SSO connection updated", "connection": {...}}
	var response struct {
		Message    string        `json:"message"`
		Connection SSOConnection `json:"connection"`
	}
	path := fmt.Sprintf("%s/%s", SSOConnectionsBasePath, uuid)
	if err := c.doRequest(ctx, http.MethodPut, path, req, &response); err != nil {
		return nil, fmt.Errorf("failed to update SSO connection %s: %w", uuid, err)
	}
	return &response.Connection, nil
}

// DeleteSSOConnection deletes an OpenID Connect connection and stops its
// billing. The API refuses while a status page still uses it.
func (c *Client) DeleteSSOConnection(ctx context.Context, uuid string) error {
	if err := ValidateResourceID(uuid); err != nil {
		return fmt.Errorf("DeleteSSOConnection: %w", err)
	}

	path := fmt.Sprintf("%s/%s", SSOConnectionsBasePath, uuid)
	if err := c.doRequest(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("failed to delete SSO connection %s: %w", uuid, err)
	}
	return nil
}
