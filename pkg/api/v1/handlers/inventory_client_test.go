package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/metahub-registry/metahub-api/pkg/config"
)

func testClient(t *testing.T, handler http.HandlerFunc, mutate func(*config.InventoryResolutionConfig)) (*InventoryClient, *int) {
	t.Helper()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		handler(w, r)
	}))
	t.Cleanup(srv.Close)

	cfg := config.InventoryResolutionConfig{
		URL:      srv.URL,
		Timeout:  2 * time.Second,
		CacheTTL: 60 * time.Second,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	return NewInventoryClient(cfg), &calls
}

// --- contract ----------------------------------------------------------------

func TestInventoryResolveHit(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/resolve", r.URL.Path)
		assert.Equal(t, "10.10.1.42", r.URL.Query().Get("ip"))
		w.Write([]byte(`{"ip":"10.10.1.42","profile":"gpu/a100","hostname":"gpu-h100","source":"ip:0"}`))
	}, nil)

	profile, ok := c.Resolve(context.Background(), "10.10.1.42")
	assert.True(t, ok)
	assert.Equal(t, "gpu/a100", profile)
}

func TestInventoryResolve404NegativeCached(t *testing.T) {
	c, calls := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{}`))
	}, nil)

	for i := 0; i < 3; i++ {
		profile, ok := c.Resolve(context.Background(), "10.10.1.42")
		assert.False(t, ok)
		assert.Empty(t, profile)
	}
	assert.Equal(t, 1, *calls, "404 must be negative-cached for cache_ttl")
}

func TestInventoryResolveHitCached(t *testing.T) {
	c, calls := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ip":"10.10.1.42","profile":"gpu/a100","source":"ip:0"}`))
	}, nil)

	for i := 0; i < 3; i++ {
		profile, ok := c.Resolve(context.Background(), "10.10.1.42")
		assert.True(t, ok)
		assert.Equal(t, "gpu/a100", profile)
	}
	assert.Equal(t, 1, *calls, "cache hit must avoid further HTTP calls")
}

func TestInventoryResolveCacheExpires(t *testing.T) {
	c, calls := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ip":"10.0.0.1","profile":"zen4","source":"cidr:0"}`))
		// note: ttl mutated below
	}, nil)
	start := time.Now()
	c.now = func() time.Time { return start }
	c.ttl = time.Minute

	_, _ = c.Resolve(context.Background(), "10.0.0.1")
	_, _ = c.Resolve(context.Background(), "10.0.0.1")
	assert.Equal(t, 1, *calls)

	// Advance past the TTL → lazy expiry triggers a fresh call.
	c.now = func() time.Time { return start.Add(2 * time.Minute) }
	_, _ = c.Resolve(context.Background(), "10.0.0.1")
	assert.Equal(t, 2, *calls)
}

// --- fail-open contract ------------------------------------------------------

func TestInventoryResolveTimeout(t *testing.T) {
	c, calls := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.Write([]byte(`{"ip":"x","profile":"late","source":"ip:0"}`))
	}, func(cfg *config.InventoryResolutionConfig) {
		cfg.Timeout = 5 * time.Millisecond // request dies before the handler answers
	})

	profile, ok := c.Resolve(context.Background(), "10.0.0.1")
	assert.False(t, ok, "timeout must fail open")
	assert.Empty(t, profile)
	assert.Equal(t, 1, *calls)
}

func TestInventoryResolveConnectionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	cfg := config.InventoryResolutionConfig{URL: srv.URL, Timeout: time.Second, CacheTTL: time.Second}
	srv.Close() // nothing listens anymore

	c := NewInventoryClient(cfg)
	profile, ok := c.Resolve(context.Background(), "10.0.0.1")
	assert.False(t, ok)
	assert.Empty(t, profile)
}

func TestInventoryResolveMalformedBody(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`this is not json`))
	}, nil)

	profile, ok := c.Resolve(context.Background(), "10.0.0.1")
	assert.False(t, ok)
	assert.Empty(t, profile)
}

func TestInventoryResolveEmptyProfile(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ip":"10.0.0.1","profile":"","source":"cidr:0"}`))
	}, nil)

	profile, ok := c.Resolve(context.Background(), "10.0.0.1")
	assert.False(t, ok, "a 200 with an empty profile is a no-match")
	assert.Empty(t, profile)
}

func TestInventoryResolveUnexpectedStatus(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}, nil)

	profile, ok := c.Resolve(context.Background(), "10.0.0.1")
	assert.False(t, ok, "non-200 ⇒ no-match (never propagate a 5xx)")
	assert.Empty(t, profile)
}

// --- token + disabled states -------------------------------------------------

func TestInventoryResolveSendsBearerToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"ip":"10.0.0.1","profile":"zen4","source":"cidr:0"}`))
	}))
	defer srv.Close()

	t.Setenv("TEST_INVENTORY_TOKEN_ABC", "sekrit")
	c := NewInventoryClient(config.InventoryResolutionConfig{
		URL:      srv.URL,
		TokenEnv: "TEST_INVENTORY_TOKEN_ABC",
		Timeout:  2 * time.Second,
		CacheTTL: 60 * time.Second,
	})
	profile, ok := c.Resolve(context.Background(), "10.0.0.1")
	assert.True(t, ok)
	assert.Equal(t, "zen4", profile)
	assert.Equal(t, "Bearer sekrit", gotAuth)
}

func TestInventoryResolveOmitsAuthHeaderWithoutToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"ip":"10.0.0.1","profile":"zen4","source":"cidr:0"}`))
	}))
	defer srv.Close()

	c := NewInventoryClient(config.InventoryResolutionConfig{
		URL: srv.URL, Timeout: 2 * time.Second, CacheTTL: 60 * time.Second,
	})
	_, ok := c.Resolve(context.Background(), "10.0.0.1")
	assert.True(t, ok)
	assert.Empty(t, gotAuth, "no token_env ⇒ no Authorization header (network trust)")
}

func TestInventoryNilClientShortCircuits(t *testing.T) {
	var c *InventoryClient
	profile, ok := c.Resolve(context.Background(), "10.0.0.1")
	assert.False(t, ok, "nil (disabled) client must short-circuit, not panic")
	assert.Empty(t, profile)
}

func TestInventoryResolveEmptyIP(t *testing.T) {
	c, calls := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ip":"","profile":"x","source":"ip:0"}`))
	}, nil)

	profile, ok := c.Resolve(context.Background(), "")
	assert.False(t, ok)
	assert.Empty(t, profile)
	assert.Equal(t, 0, *calls, "empty ip must not hit the service")
}