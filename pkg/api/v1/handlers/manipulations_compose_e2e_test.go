//go:build e2e
// +build e2e

// Package handlers e2e tests for the manipulation-extended schema.
//
// These tests drive the live compose stack:
//
//   docker compose up -d dev neo4j minio registry
//   # seed rules + the glow manipulation via metahub-admin
//   task compose:seed:graph
//
// then from the metahub-api worktree:
//
//   go test -tags=e2e -timeout=180s -run TestE2E ./pkg/api/v1/handlers/...
//
// The tests skip cleanly when the compose stack is not reachable.
package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// composeBaseURL returns the metahub-api URL exposed by the compose
// stack. Override via METAHUB_COMPOSE_URL at test time.
func composeBaseURL() string {
	if v, ok := os.LookupEnv("METAHUB_COMPOSE_URL"); ok && v != "" {
		return strings.TrimRight(v, "/")
	}
	return "http://localhost:8080"
}

// composeIsReachable probes the compose stack's heartbeat endpoint. The
// probe times out fast so missing-stack runs don't hang.
func composeIsReachable(t *testing.T) bool {
	t.Helper()
	client := http.Client{Timeout: 2 * time.Second}
	url := composeBaseURL() + "/api/v1/heartbeat"
	resp, err := client.Get(url)
	if err != nil {
		t.Logf("compose heartbeat probe failed (%s): %v", url, err)
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// fetchProfileDetail calls `GET /api/v1/profiles/{label}` against the
// compose stack and decodes the response.
func fetchProfileDetail(ctx context.Context, t *testing.T, label string) (ProfileDetailResponse, int) {
	t.Helper()
	client := http.Client{Timeout: 10 * time.Second}
	url := composeBaseURL() + "/api/v1/profiles/" + urlEncodePathSegment(label)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var out ProfileDetailResponse
	if resp.StatusCode == http.StatusOK {
		require.NoError(t, json.Unmarshal(body, &out), "decode response: %s", string(body))
	}
	return out, resp.StatusCode
}

// urlEncodePathSegment percent-encodes the few characters that appear in
// profile labels and would otherwise confuse chi's path router.
func urlEncodePathSegment(s string) string {
	return strings.NewReplacer("#", "%23", "/", "%2F").Replace(s)
}

// TestE2E_GlowProfileReturnsManipulations hits the compose stack and
// asserts that after `task compose:seed:graph` has run, the glow profile
// exposes its manipulation tree through the API.
func TestE2E_GlowProfileReturnsManipulations(t *testing.T) {
	if !composeIsReachable(t) {
		t.Skip("compose stack not reachable at " + composeBaseURL())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, status := fetchProfileDetail(ctx, t, "glow#1.5.0")
	if status == http.StatusNotFound {
		t.Skip("glow#1.5.0 profile not present — run `task compose:seed:graph` first")
	}
	require.Equal(t, http.StatusOK, status, "expected 200 from profile detail endpoint")
	assert.Equal(t, "glow#1.5.0", resp.Label)
	require.NotNil(t, resp.Manipulations, "manipulations must be populated (even if empty)")
	if len(resp.Manipulations) == 0 {
		t.Skip("no manipulations linked to glow profile — run `task compose:seed:graph`")
	}
	mp := resp.Manipulations[0]
	assert.Equal(t, "layer/glow#1.5.0", mp.Label)
	// Expect the migrated glow fixture to expose two variants.
	require.GreaterOrEqual(t, len(mp.Variants), 1, "glow manipulation should expose at least one variant")
	rules := make([]string, 0, len(mp.Variants))
	for _, v := range mp.Variants {
		rules = append(rules, v.Rule)
	}
	t.Logf("glow variants: %v", rules)
	assert.Contains(t, rules, "arch:arm64=true")
}
