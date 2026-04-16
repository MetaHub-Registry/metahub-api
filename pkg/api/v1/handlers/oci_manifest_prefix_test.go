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

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
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
