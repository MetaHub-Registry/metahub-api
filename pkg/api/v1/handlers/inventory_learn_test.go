package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/metahub-registry/metahub-api/pkg/config"
)

// learnStub records learn POSTs against /api/v1/entries.
type learnStub struct {
	mu    sync.Mutex
	bodies []map[string]any
	statuses []int
}

func newLearnServer(t *testing.T, statuses ...int) (*httptest.Server, *learnStub) {
	t.Helper()
	st := &learnStub{statuses: statuses}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		defer st.mu.Unlock()
		idx := len(st.bodies)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		st.bodies = append(st.bodies, body)
		code := http.StatusCreated
		if idx < len(st.statuses) {
			code = st.statuses[idx]
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_, _ = w.Write([]byte(`{"status":"learned"}`))
	}))
	t.Cleanup(srv.Close)
	return srv, st
}

func newLearnClient(t *testing.T, srvURL string) *InventoryClient {
	t.Helper()
	return NewInventoryClient(config.InventoryResolutionConfig{
		URL: srvURL, Timeout: 2 * time.Second, CacheTTL: time.Minute,
	})
}

func TestInventoryLearnAccepted(t *testing.T) {
	srv, st := newLearnServer(t, http.StatusCreated, http.StatusOK)
	c := newLearnClient(t, srv.URL)

	require.True(t, c.Learn(context.Background(), "192.168.5.115", "frame"))
	require.True(t, c.Learn(context.Background(), "192.168.5.115", "zen4"))
	require.Len(t, st.bodies, 2)
	assert.Equal(t, "192.168.5.115", st.bodies[0]["ip"])
	assert.Equal(t, "frame", st.bodies[0]["profile"])
	assert.Equal(t, true, st.bodies[0]["learned"])
}

func TestInventoryLearnFailure(t *testing.T) {
	srv, _ := newLearnServer(t, http.StatusInternalServerError)
	c := newLearnClient(t, srv.URL)
	assert.False(t, c.Learn(context.Background(), "10.0.0.1", "x"))
}

func TestInventoryLearnClearsNegativeCache(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"status":"learned"}`))
		default:
			calls++
			if calls == 1 {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{}`))
				return
			}
			_, _ = w.Write([]byte(`{"ip":"10.9.9.9","profile":"node/aws","source":"learned:10.9.9.9"}`))
		}
	}))
	defer srv.Close()
	c := newLearnClient(t, srv.URL)

	// Suffix-less login → negative cached.
	_, ok := c.Resolve(context.Background(), "10.9.9.9")
	require.False(t, ok)
	// (still cached)
	_, ok = c.Resolve(context.Background(), "10.9.9.9")
	require.False(t, ok)

	// Operator does an explicit login → learn → cache cleared.
	require.True(t, c.Learn(context.Background(), "10.9.9.9", "node/aws"))

	// Next suffix-less login re-resolves and finds the learned profile.
	profile, ok := c.Resolve(context.Background(), "10.9.9.9")
	require.True(t, ok, "negative cache must have been cleared by Learn")
	assert.Equal(t, "node/aws", profile)
}

func TestInventoryLearnNilAndEmpty(t *testing.T) {
	var c *InventoryClient
	assert.False(t, c.Learn(context.Background(), "10.0.0.1", "x"), "nil receiver")
	c2 := newLearnClient(t, "http://127.0.0.1:1") // nothing listens
	assert.False(t, c2.Learn(context.Background(), "", "x"), "empty ip")
}

func TestInventoryLearnAsyncEventuallyPosts(t *testing.T) {
	srv, st := newLearnServer(t, http.StatusCreated)
	c := newLearnClient(t, srv.URL)

	c.LearnAsync("192.168.5.115", "frame")

	require.Eventually(t, func() bool {
		st.mu.Lock()
		defer st.mu.Unlock()
		return len(st.bodies) == 1
	}, 2*time.Second, 20*time.Millisecond, "async learn must reach the inventory")
	assert.Equal(t, "frame", st.bodies[0]["profile"])
}
