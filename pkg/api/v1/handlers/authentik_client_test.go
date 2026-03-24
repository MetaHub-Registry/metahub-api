package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockAuthentikAPI(t *testing.T) (*httptest.Server, *AuthentikClient) {
	t.Helper()

	// Use chi for the mock to support path params
	r := chi.NewRouter()

	// POST /core/tokens/ — create token
	r.Post("/core/tokens/", func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer valid-user-token" {
			http.Error(w, `{"detail":"Authentication credentials were not provided."}`, http.StatusForbidden)
			return
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			http.Error(w, `{"detail":"bad request"}`, http.StatusBadRequest)
			return
		}
		resp := AuthentikToken{
			Identifier:  payload["identifier"].(string),
			Description: payload["description"].(string),
			Intent:      "app_password",
		}
		resp.UserObj.Username = "alice"
		resp.UserObj.PK = 1
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	})

	// GET /core/tokens/ — list tokens
	r.Get("/core/tokens/", func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer valid-user-token" {
			http.Error(w, `{"detail":"Authentication credentials were not provided."}`, http.StatusForbidden)
			return
		}
		resp := AuthentikTokenList{
			Results: []AuthentikToken{
				{Identifier: "metahub-registry-111", Description: "laptop", Intent: "app_password"},
				{Identifier: "metahub-registry-222", Description: "ci", Intent: "app_password"},
			},
		}
		resp.Pagination.Count = 2
		resp.Pagination.Current = 1
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// GET /core/tokens/{id}/view_key/ — view token key
	r.Get("/core/tokens/{id}/view_key/", func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer valid-user-token" {
			http.Error(w, `{"detail":"forbidden"}`, http.StatusForbidden)
			return
		}
		id := chi.URLParam(req, "id")
		if id == "nonexistent" {
			http.Error(w, `{"detail":"Not found."}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AuthentikTokenKey{Key: "secret-app-password-key"})
	})

	// DELETE /core/tokens/{id}/
	r.Delete("/core/tokens/{id}/", func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer valid-user-token" {
			http.Error(w, `{"detail":"forbidden"}`, http.StatusForbidden)
			return
		}
		id := chi.URLParam(req, "id")
		if id == "nonexistent" || strings.HasSuffix(id, "nonexistent") {
			http.Error(w, `{"detail":"Not found."}`, http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	client := NewAuthentikClient(srv.URL)
	return srv, client
}

func TestCreateAppPassword_Success(t *testing.T) {
	_, client := newMockAuthentikAPI(t)
	ctx := context.Background()
	token, err := client.CreateAppPassword(ctx, "valid-user-token", "metahub-registry-123", "my laptop")
	require.NoError(t, err)
	assert.Equal(t, "metahub-registry-123", token.Identifier)
	assert.Equal(t, "my laptop", token.Description)
	assert.Equal(t, "app_password", token.Intent)
	assert.Equal(t, "alice", token.UserObj.Username)
}

func TestCreateAppPassword_Forbidden(t *testing.T) {
	_, client := newMockAuthentikAPI(t)
	ctx := context.Background()
	_, err := client.CreateAppPassword(ctx, "bad-token", "metahub-registry-123", "test")
	require.Error(t, err)
	var authErr *AuthentikError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, http.StatusForbidden, authErr.StatusCode)
}

func TestViewTokenKey_Success(t *testing.T) {
	_, client := newMockAuthentikAPI(t)
	ctx := context.Background()
	key, err := client.ViewTokenKey(ctx, "valid-user-token", "metahub-registry-111")
	require.NoError(t, err)
	assert.Equal(t, "secret-app-password-key", key)
}

func TestViewTokenKey_NotFound(t *testing.T) {
	_, client := newMockAuthentikAPI(t)
	ctx := context.Background()
	_, err := client.ViewTokenKey(ctx, "valid-user-token", "nonexistent")
	require.Error(t, err)
	var authErr *AuthentikError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, http.StatusNotFound, authErr.StatusCode)
}

func TestListAppPasswords_Success(t *testing.T) {
	_, client := newMockAuthentikAPI(t)
	ctx := context.Background()
	tokens, err := client.ListAppPasswords(ctx, "valid-user-token", "metahub-registry-")
	require.NoError(t, err)
	assert.Len(t, tokens, 2)
	assert.Equal(t, "metahub-registry-111", tokens[0].Identifier)
	assert.Equal(t, "metahub-registry-222", tokens[1].Identifier)
}

func TestListAppPasswords_Forbidden(t *testing.T) {
	_, client := newMockAuthentikAPI(t)
	ctx := context.Background()
	_, err := client.ListAppPasswords(ctx, "bad-token", "metahub-registry-")
	require.Error(t, err)
	var authErr *AuthentikError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, http.StatusForbidden, authErr.StatusCode)
}

func TestRevokeAppPassword_Success(t *testing.T) {
	_, client := newMockAuthentikAPI(t)
	ctx := context.Background()
	err := client.RevokeAppPassword(ctx, "valid-user-token", "metahub-registry-111")
	require.NoError(t, err)
}

func TestRevokeAppPassword_NotFound(t *testing.T) {
	_, client := newMockAuthentikAPI(t)
	ctx := context.Background()
	err := client.RevokeAppPassword(ctx, "valid-user-token", "nonexistent")
	require.Error(t, err)
	var authErr *AuthentikError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, http.StatusNotFound, authErr.StatusCode)
}

func TestAuthentikError_ErrorMessage(t *testing.T) {
	err := &AuthentikError{StatusCode: 403, Body: "forbidden"}
	assert.Contains(t, err.Error(), "403")
	assert.Contains(t, err.Error(), "forbidden")
}

func TestAuthentikClient_NilHTTPClient(t *testing.T) {
	// Test that a client with nil HTTPClient falls back to http.DefaultClient
	client := &AuthentikClient{BaseURL: "http://localhost:9999"}
	ctx := context.Background()
	_, err := client.ListAppPasswords(ctx, "token", "prefix")
	// Should fail to connect but not panic
	require.Error(t, err)
}
