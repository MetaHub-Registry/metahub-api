package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	errDecodeResponse = "failed to decode response: %w"
)

// AuthentikClient provides methods to interact with Authentik's core REST API
// for managing app password tokens. All methods require a user-scoped access
// token (Bearer) that has the goauthentik.io/api scope.
type AuthentikClient struct {
	BaseURL    string       // e.g. "https://authentik.example.com/api/v3"
	HTTPClient *http.Client // optional; defaults to a 10s-timeout client
}

// NewAuthentikClient creates a client for the given Authentik API base URL.
func NewAuthentikClient(baseURL string) *AuthentikClient {
	return &AuthentikClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// AuthentikToken represents a token object returned by Authentik's core API.
type AuthentikToken struct {
	Identifier  string `json:"identifier"`
	Description string `json:"description"`
	Intent      string `json:"intent"`
	Expiring    bool   `json:"expiring"`
	Expires     string `json:"expires,omitempty"`
	UserObj     struct {
		Username string `json:"username"`
		PK       int    `json:"pk"`
	} `json:"user_obj"`
}

// AuthentikTokenKey holds the secret key value retrieved from view_key.
type AuthentikTokenKey struct {
	Key string `json:"key"`
}

// AuthentikTokenList is the paginated response from GET /core/tokens/.
type AuthentikTokenList struct {
	Pagination struct {
		Count   int `json:"count"`
		Current int `json:"current"`
	} `json:"pagination"`
	Results []AuthentikToken `json:"results"`
}

// AuthentikError is returned when the API responds with an error.
type AuthentikError struct {
	StatusCode int
	Body       string
}

func (e *AuthentikError) Error() string {
	return fmt.Sprintf("authentik API error (HTTP %d): %s", e.StatusCode, e.Body)
}

// doRequest executes an HTTP request against the Authentik API with the user's token.
func (c *AuthentikClient) doRequest(ctx context.Context, method, path string, userToken string, body io.Reader) (*http.Response, error) {
	url := c.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return client.Do(req)
}

// checkResponse reads and checks the HTTP response. Returns an error for non-2xx status codes.
func checkResponse(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return &AuthentikError{
		StatusCode: resp.StatusCode,
		Body:       string(body),
	}
}

// CreateAppPassword creates a new app password token in Authentik for the authenticated user.
// The identifier should be unique (e.g., "metahub-registry-<timestamp>").
//
// ttl controls the token lifetime. A zero ttl creates a NON-expiring token
// (expiring=false), so registry credentials do not inherit Authentik's short
// tenant default_token_duration. A positive ttl creates an expiring token with
// expires = now + ttl (RFC3339, UTC).
func (c *AuthentikClient) CreateAppPassword(ctx context.Context, userToken, identifier, description string, ttl time.Duration) (*AuthentikToken, error) {
	payload := map[string]interface{}{
		"identifier":  identifier,
		"intent":      "app_password",
		"description": description,
	}
	if ttl > 0 {
		payload["expiring"] = true
		payload["expires"] = time.Now().UTC().Add(ttl).Format(time.RFC3339)
	} else {
		payload["expiring"] = false
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	resp, err := c.doRequest(ctx, http.MethodPost, "/core/tokens/", userToken, strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to create app password: %w", err)
	}
	defer resp.Body.Close()
	if err := checkResponse(resp); err != nil {
		return nil, err
	}
	var token AuthentikToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf(errDecodeResponse, err)
	}
	return &token, nil
}

// ViewTokenKey retrieves the secret key value of an app password token.
// The key is only available via this endpoint (not in the list response).
func (c *AuthentikClient) ViewTokenKey(ctx context.Context, userToken, identifier string) (string, error) {
	path := fmt.Sprintf("/core/tokens/%s/view_key/", identifier)
	resp, err := c.doRequest(ctx, http.MethodGet, path, userToken, nil)
	if err != nil {
		return "", fmt.Errorf("failed to view token key: %w", err)
	}
	defer resp.Body.Close()
	if err := checkResponse(resp); err != nil {
		return "", err
	}
	var key AuthentikTokenKey
	if err := json.NewDecoder(resp.Body).Decode(&key); err != nil {
		return "", fmt.Errorf(errDecodeResponse, err)
	}
	return key.Key, nil
}

// ListAppPasswords lists app password tokens with identifiers starting with the given prefix.
func (c *AuthentikClient) ListAppPasswords(ctx context.Context, userToken, prefix string) ([]AuthentikToken, error) {
	path := fmt.Sprintf("/core/tokens/?intent=app_password&identifier__startswith=%s", prefix)
	resp, err := c.doRequest(ctx, http.MethodGet, path, userToken, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list app passwords: %w", err)
	}
	defer resp.Body.Close()
	if err := checkResponse(resp); err != nil {
		return nil, err
	}
	var list AuthentikTokenList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf(errDecodeResponse, err)
	}
	return list.Results, nil
}

// RevokeAppPassword deletes an app password token by identifier.
func (c *AuthentikClient) RevokeAppPassword(ctx context.Context, userToken, identifier string) error {
	path := fmt.Sprintf("/core/tokens/%s/", identifier)
	resp, err := c.doRequest(ctx, http.MethodDelete, path, userToken, nil)
	if err != nil {
		return fmt.Errorf("failed to revoke app password: %w", err)
	}
	defer resp.Body.Close()
	if err := checkResponse(resp); err != nil {
		return err
	}
	return nil
}
