package handlers

// Unit-level tests for the prefix-preserving pullthrough helpers
// introduced by pullthrough-repo-handler (D5/D6).
//
// These tests do NOT require Minio/Neo4j — they only exercise the
// in-memory Magazine.Config.Registries lookup used by
// resolveUpstreamTarget. The contract under test: when the request
// repo starts with a configured registry hostname, the helper returns
// that hostname WITHOUT mutating the caller's repo string (prefix is
// preserved; namespace isolation from client-push repos is guaranteed
// by construction).
//
// Additional coverage for the error-path helpers writeManifestNotFound
// and writeInternalError extracted during the cognitive-complexity
// refactor, since these paths are not exercised by the happy-path
// integration tests.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// newPrefixTestHandlers builds a Handlers with a minimal in-memory Magazine
// that only populates Config.Registries. No Minio/Neo4j required.
func newPrefixTestHandlers(registries ...magazine.RegistryConfig) Handlers {
	mag := magazine.Magazine{
		Config: magazine.MagConfig{
			Registries: registries,
		},
	}
	return NewHandlers("test", mag, nil, nil)
}

func TestResolveUpstreamTarget_MatchesConfiguredRegistry(t *testing.T) {
	h := newPrefixTestHandlers(magazine.RegistryConfig{Hostname: "zot.example.com", AllowAnon: true})
	l, _, span := utils.InitLogging(context.Background(), tracer, map[string]any{"func": "TestResolveUpstreamTarget_Match"})
	defer span.End()

	input := "zot.example.com/library/alpine"
	got := h.resolveUpstreamTarget(l, span, input)

	assert.Equal(t, "zot.example.com", got, "configured registry hostname should be returned as target")
	assert.Equal(t, "zot.example.com/library/alpine", input, "input repo must NOT be mutated (prefix preserved by caller)")
}

func TestResolveUpstreamTarget_MatchesConfiguredRegistryWithPort(t *testing.T) {
	h := newPrefixTestHandlers(magazine.RegistryConfig{Hostname: "registry.internal:5000", AllowAnon: true})
	l, _, span := utils.InitLogging(context.Background(), tracer, map[string]any{"func": "TestResolveUpstreamTarget_Port"})
	defer span.End()

	input := "registry.internal:5000/library/alpine"
	got := h.resolveUpstreamTarget(l, span, input)

	assert.Equal(t, "registry.internal:5000", got, "port-bearing hostname should match as a single first segment")
}

func TestResolveUpstreamTarget_NoMatchReturnsEmpty(t *testing.T) {
	h := newPrefixTestHandlers(magazine.RegistryConfig{Hostname: "zot.example.com", AllowAnon: true})
	l, _, span := utils.InitLogging(context.Background(), tracer, map[string]any{"func": "TestResolveUpstreamTarget_NoMatch"})
	defer span.End()

	// "myorg" is not a configured registry — treat as local repo.
	got := h.resolveUpstreamTarget(l, span, "myorg/myimage")

	assert.Equal(t, "", got, "unknown prefix must not match any configured registry")
}

func TestResolveUpstreamTarget_EmptyRegistriesReturnsEmpty(t *testing.T) {
	h := newPrefixTestHandlers() // no registries configured
	l, _, span := utils.InitLogging(context.Background(), tracer, map[string]any{"func": "TestResolveUpstreamTarget_EmptyCfg"})
	defer span.End()

	got := h.resolveUpstreamTarget(l, span, "zot.example.com/library/alpine")

	assert.Equal(t, "", got, "with no registries configured, no prefix should match")
}

func TestResolveUpstreamTarget_PreservesCallerRepoAcrossCalls(t *testing.T) {
	// Regression guard for D5/D6: calling resolveUpstreamTarget repeatedly on
	// the same string must never mutate the string. This is what guarantees
	// "namespace isolation from client-push repos by construction" — if the
	// helper ever starts mutating, the contract breaks silently.
	h := newPrefixTestHandlers(magazine.RegistryConfig{Hostname: "zot.example.com", AllowAnon: true})
	l, _, span := utils.InitLogging(context.Background(), tracer, map[string]any{"func": "TestResolveUpstreamTarget_NoMutate"})
	defer span.End()

	input := "zot.example.com/library/alpine"
	for i := 0; i < 3; i++ {
		_ = h.resolveUpstreamTarget(l, span, input)
		assert.Equal(t, "zot.example.com/library/alpine", input, "iteration %d: input must be unchanged", i)
	}
}

// TestWriteManifestNotFound asserts the shape of the 404 MANIFEST_UNKNOWN
// response: correct status code, OCI error code, and detail payload
// referencing the requested repo/tag.
func TestWriteManifestNotFound(t *testing.T) {
	l, _, span := utils.InitLogging(context.Background(), tracer, map[string]any{"func": "TestWriteManifestNotFound"})
	defer span.End()
	req := types.Request{Repo: "qnib/ftest", Tag: "latest", Profile: "generic", Method: "GET"}
	w := httptest.NewRecorder()

	writeManifestNotFound(w, l, span, req)

	assert.Equal(t, 404, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	errs, ok := body["errors"].([]any)
	require.True(t, ok, "response body must contain errors array")
	require.Len(t, errs, 1)
	first, ok := errs[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "MANIFEST_UNKNOWN", first["code"])
	assert.Contains(t, first["message"], "qnib/ftest")
	assert.Contains(t, first["message"], "latest")
	assert.Contains(t, first["message"], "generic")
}

// TestWriteInternalError asserts the shape of the 500 INTERNAL_ERROR
// response used when ManifestExists returns an error. This path is not
// exercised by the happy-path handler integration tests.
func TestWriteInternalError(t *testing.T) {
	l, _, span := utils.InitLogging(context.Background(), tracer, map[string]any{"func": "TestWriteInternalError"})
	defer span.End()
	req := types.Request{Repo: "qnib/ftest", Tag: "latest", Profile: "generic", Method: "GET"}
	w := httptest.NewRecorder()

	writeInternalError(w, l, span, req, "simulated failure: %v", errors.New("boom"))

	assert.Equal(t, 500, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	errs, ok := body["errors"].([]any)
	require.True(t, ok)
	require.Len(t, errs, 1)
	first, ok := errs[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "INTERNAL_ERROR", first["code"])
	assert.Contains(t, first["message"], "simulated failure")
	assert.Contains(t, first["message"], "boom")
}

// TestResolveMissingManifest_NoDigestNoTag_WritesNotFound exercises the
// fallthrough path when a request has neither a digest nor a tag: both the
// Path-Z digest lookup and the pullthrough branch are skipped and the
// handler writes a straight 404 MANIFEST_UNKNOWN.
func TestResolveMissingManifest_NoDigestNoTag_WritesNotFound(t *testing.T) {
	h := newPrefixTestHandlers()
	l, ctx, span := utils.InitLogging(context.Background(), tracer, map[string]any{"func": "TestResolveMissingManifest_NoDigestNoTag"})
	defer span.End()
	req := types.Request{Repo: "qnib/ftest", Tag: "", Digest: "", Profile: "generic", Method: "GET"}
	w := httptest.NewRecorder()

	mt, stop := h.resolveMissingManifest(ctx, w, l, span, req, upstream.UpstreamTriple{}, "")

	assert.True(t, stop, "must signal stop so caller does not dispatch")
	assert.Equal(t, "", mt, "no mediaType resolved")
	assert.Equal(t, 404, w.Code)
}
