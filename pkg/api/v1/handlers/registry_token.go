package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const (
	registryTokenPrefix   = "metahub-registry-"
	errTokenNotConfigured = "registry token management not configured"
	errMissingAccessToken = "missing access token"
	errInsufficientScope  = "access token lacks required scope for token management"
)

// RegistryTokenCreateRequest is the request body for creating a registry token.
type RegistryTokenCreateRequest struct {
	Description string `json:"description"`
}

// RegistryTokenCreateResponse is returned after creating a registry token.
type RegistryTokenCreateResponse struct {
	Identifier  string `json:"identifier"`
	Description string `json:"description"`
	DockerLogin string `json:"docker_login"`
	Username    string `json:"username"`
	Password    string `json:"password"`
}

// RegistryTokenListItem represents a registry token in list responses.
type RegistryTokenListItem struct {
	Identifier  string `json:"identifier"`
	Description string `json:"description"`
}

// RegistryTokenCreate handles POST /api/v1/protected/registry-token.
// Creates an Authentik app password for the authenticated user and returns
// a ready-to-use docker login command.
func (h Handlers) RegistryTokenCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.oidc == nil || h.oidc.AuthentikAPIURL == "" {
		writeJSONError(w, http.StatusNotImplemented, errTokenNotConfigured)
		return
	}

	claims, ok := OIDCClaimsFromContext(ctx)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	rawToken, ok := RawTokenFromContext(ctx)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, errMissingAccessToken)
		return
	}

	var req RegistryTokenCreateRequest
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}
	if req.Description == "" {
		req.Description = "MetaHub Registry Access"
	}

	identifier := fmt.Sprintf("%s%d", registryTokenPrefix, time.Now().Unix())
	client := NewAuthentikClient(h.oidc.AuthentikAPIURL)

	_, err := client.CreateAppPassword(ctx, rawToken, identifier, req.Description)
	if err != nil {
		if authErr, ok := err.(*AuthentikError); ok && authErr.StatusCode == http.StatusForbidden {
			writeJSONError(w, http.StatusForbidden, errInsufficientScope)
			return
		}
		writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("failed to create registry token: %v", err))
		return
	}

	key, err := client.ViewTokenKey(ctx, rawToken, identifier)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("token created but failed to retrieve key: %v", err))
		return
	}

	// Build the docker login command using the registry host from the request
	registryHost := r.Host
	if registryHost == "" {
		registryHost = "registry.example.com"
	}
	dockerLogin := fmt.Sprintf("docker login -u %s -p %s %s", claims.Username, key, registryHost)

	resp := RegistryTokenCreateResponse{
		Identifier:  identifier,
		Description: req.Description,
		DockerLogin: dockerLogin,
		Username:    claims.Username,
		Password:    key,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// RegistryTokenList handles GET /api/v1/protected/registry-tokens.
// Lists the authenticated user's registry tokens (app passwords with metahub-registry- prefix).
func (h Handlers) RegistryTokenList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.oidc == nil || h.oidc.AuthentikAPIURL == "" {
		writeJSONError(w, http.StatusNotImplemented, errTokenNotConfigured)
		return
	}

	rawToken, ok := RawTokenFromContext(ctx)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, errMissingAccessToken)
		return
	}

	client := NewAuthentikClient(h.oidc.AuthentikAPIURL)
	tokens, err := client.ListAppPasswords(ctx, rawToken, registryTokenPrefix)
	if err != nil {
		if authErr, ok := err.(*AuthentikError); ok && authErr.StatusCode == http.StatusForbidden {
			writeJSONError(w, http.StatusForbidden, errInsufficientScope)
			return
		}
		writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("failed to list registry tokens: %v", err))
		return
	}

	items := make([]RegistryTokenListItem, 0, len(tokens))
	for _, t := range tokens {
		items = append(items, RegistryTokenListItem{
			Identifier:  t.Identifier,
			Description: t.Description,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

// RegistryTokenRevoke handles DELETE /api/v1/protected/registry-tokens/{identifier}.
// Revokes the specified registry token via Authentik's API.
func (h Handlers) RegistryTokenRevoke(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.oidc == nil || h.oidc.AuthentikAPIURL == "" {
		writeJSONError(w, http.StatusNotImplemented, errTokenNotConfigured)
		return
	}

	identifier := chi.URLParam(r, "identifier")
	if !strings.HasPrefix(identifier, registryTokenPrefix) {
		writeJSONError(w, http.StatusBadRequest, "invalid registry token identifier")
		return
	}

	rawToken, ok := RawTokenFromContext(ctx)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, errMissingAccessToken)
		return
	}

	client := NewAuthentikClient(h.oidc.AuthentikAPIURL)
	err := client.RevokeAppPassword(ctx, rawToken, identifier)
	if err != nil {
		if authErr, ok := err.(*AuthentikError); ok {
			switch authErr.StatusCode {
			case http.StatusNotFound:
				writeJSONError(w, http.StatusNotFound, "token not found")
				return
			case http.StatusForbidden:
				writeJSONError(w, http.StatusForbidden, errInsufficientScope)
				return
			}
		}
		writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("failed to revoke registry token: %v", err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
