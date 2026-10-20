package handlers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"

	"github.com/metahub-registry/metahub-api/pkg/config"
)

// inventoryStub is an httptest resolve endpoint with a call counter — used to
// assert WHEN resolution is consulted and when it is skipped.
type inventoryStub struct {
	srv        *httptest.Server
	calls      int // resolve (GET) calls
	learnCalls int // learned-entry reports (POST /api/v1/entries)
	respond    func(w http.ResponseWriter, r *http.Request)
}

func newInventoryStub(t *testing.T, respond func(w http.ResponseWriter, r *http.Request)) *inventoryStub {
	t.Helper()
	st := &inventoryStub{respond: respond}
	st.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/entries" {
			st.learnCalls++
		} else {
			st.calls++
		}
		st.respond(w, r)
	}))
	t.Cleanup(st.srv.Close)
	return st
}

func (st *inventoryStub) client() *InventoryClient {
	return NewInventoryClient(config.InventoryResolutionConfig{
		URL:      st.srv.URL,
		Timeout:  2 * time.Second,
		CacheTTL: time.Second,
	})
}

func resolveHit(profile string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ip":"192.168.5.115","profile":"` + profile + `","hostname":"frame.hvk","source":"ip:0"}`))
	}
}

// --- god-mode path ----------------------------------------------------------

func TestAuthHandler_InventoryResolvesGodModeToken(t *testing.T) {
	st := newInventoryStub(t, resolveHit("frame"))
	h := Handlers{mag: magazine.Magazine{}, godmode: newTestGodMode()}.
		WithInventory(st.client())

	// Suffix-less login from a LAN node: ?account=oci (no profile suffix) and
	// client IP 192.168.5.115 (frame, per the dev inventory seed).
	r := httptest.NewRequest("GET", "/auth?account=oci", nil)
	r.Header.Set("X-Forwarded-For", "192.168.5.115")
	r.SetBasicAuth("oci", "oci-pass")
	w := httptest.NewRecorder()
	h.AuthHandler(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	claims, err := h.godmode.VerifyToken(resp["token"])
	require.NoError(t, err)
	// The minted token must carry the resolved suffix.
	assert.Equal(t, "oci/frame", claims.Username, "preferred_username carries the resolved suffix")
	assert.Equal(t, "oci/frame", claims.Sub)
	assert.Equal(t, 1, st.calls)
}

func TestAuthHandler_InventoryNoMatchStaysGeneric(t *testing.T) {
	st := newInventoryStub(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{}`))
	})
	h := Handlers{mag: magazine.Magazine{}, godmode: newTestGodMode()}.
		WithInventory(st.client())

	r := httptest.NewRequest("GET", "/auth?account=oci", nil)
	r.Header.Set("X-Forwarded-For", "10.9.9.9")
	r.SetBasicAuth("oci", "oci-pass")
	w := httptest.NewRecorder()
	h.AuthHandler(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	claims, err := h.godmode.VerifyToken(resp["token"])
	require.NoError(t, err)
	// Fail-open: no-match ⇒ token stays suffix-less (legacy behaviour).
	assert.Equal(t, "oci", claims.Username)
}

func TestAuthHandler_InventoryOutageFailsOpen(t *testing.T) {
	// Client pointed at a dead endpoint: connection error ⇒ generic, never 5xx.
	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	deadURL := dead.URL
	dead.Close()
	c := NewInventoryClient(config.InventoryResolutionConfig{
		URL: deadURL, Timeout: 500 * time.Millisecond, CacheTTL: time.Second,
	})
	h := Handlers{mag: magazine.Magazine{}, godmode: newTestGodMode()}.WithInventory(c)

	r := httptest.NewRequest("GET", "/auth?account=oci", nil)
	r.SetBasicAuth("oci", "oci-pass")
	w := httptest.NewRecorder()
	h.AuthHandler(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	claims, err := h.godmode.VerifyToken(resp["token"])
	require.NoError(t, err)
	assert.Equal(t, "oci", claims.Username, "inventory outage must degrade to generic")
}

func TestAuthHandler_ExplicitSuffixSkipsResolution(t *testing.T) {
	// The inventory endpoint fails the test if it is EVER called: an explicit
	// user/profile suffix must win before resolution is consulted.
	st := newInventoryStub(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("inventory consulted despite explicit profile suffix")
		w.Write([]byte(`{"profile":"should-not-apply","source":"ip:0"}`))
	})
	h := Handlers{mag: magazine.Magazine{}, godmode: newTestGodMode()}.
		WithInventory(st.client())

	r := httptest.NewRequest("GET", "/auth?account=oci/zen2", nil)
	r.SetBasicAuth("oci", "oci-pass")
	w := httptest.NewRecorder()
	h.AuthHandler(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	claims, err := h.godmode.VerifyToken(resp["token"])
	require.NoError(t, err)
	assert.Equal(t, "oci/zen2", claims.Username, "explicit suffix must be preserved untouched")
	assert.Equal(t, 0, st.calls, "resolution must be skipped entirely")
}

func TestAuthHandler_ConfigAbsentLegacyBehaviour(t *testing.T) {
	// No inventory client wired (config absent): byte-identical legacy path —
	// there is nothing that could even reach an inventory endpoint.
	h := Handlers{mag: magazine.Magazine{}, godmode: newTestGodMode()} // no WithInventory

	r := httptest.NewRequest("GET", "/auth?account=oci", nil)
	r.SetBasicAuth("oci", "oci-pass")
	w := httptest.NewRecorder()
	h.AuthHandler(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	claims, err := h.godmode.VerifyToken(resp["token"])
	require.NoError(t, err)
	assert.Equal(t, "oci", claims.Username)
}

// --- learning (openspec add-learned-entries) --------------------------------

func TestAuthHandler_ExplicitLoginLearns(t *testing.T) {
	learned := make(chan map[string]any, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		learned <- body
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"learned"}`))
	}))
	defer srv.Close()
	inv := NewInventoryClient(config.InventoryResolutionConfig{
		URL: srv.URL, Timeout: 2 * time.Second, CacheTTL: time.Minute,
	})

	// Explicit suffix → god-mode token → learn reported for the client IP.
	h := Handlers{mag: magazine.Magazine{}, godmode: newTestGodMode()}.WithInventory(inv)
	r := httptest.NewRequest("GET", "/auth?account=oci/frame", nil)
	r.SetBasicAuth("oci", "oci-pass")
	w := httptest.NewRecorder()
	h.AuthHandler(w, r)
	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	select {
	case body = <-learned:
	case <-time.After(2 * time.Second):
		t.Fatal("learn was not reported for an explicit login")
	}
	assert.Equal(t, "frame", body["profile"])
	assert.Equal(t, true, body["learned"])
	assert.NotEmpty(t, body["ip"])
}

func TestAuthHandler_SuffixlessDoesNotLearn(t *testing.T) {
	learnCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			learnCalled = true
			w.WriteHeader(http.StatusCreated)
			return
		}
		// resolve path: no match for the client IP
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	inv := NewInventoryClient(config.InventoryResolutionConfig{
		URL: srv.URL, Timeout: 500 * time.Millisecond, CacheTTL: time.Minute,
	})

	h := Handlers{mag: magazine.Magazine{}, godmode: newTestGodMode()}.WithInventory(inv)
	r := httptest.NewRequest("GET", "/auth?account=oci", nil)
	r.SetBasicAuth("oci", "oci-pass")
	w := httptest.NewRecorder()
	h.AuthHandler(w, r)
	require.Equal(t, http.StatusOK, w.Code)

	time.Sleep(300 * time.Millisecond)
	assert.False(t, learnCalled, "suffix-less logins must not learn")
}

func TestAuthHandler_LearnFailureIsSilent(t *testing.T) {
	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	deadURL := dead.URL
	dead.Close()
	inv := NewInventoryClient(config.InventoryResolutionConfig{
		URL: deadURL, Timeout: 300 * time.Millisecond, CacheTTL: time.Minute,
	})
	h := Handlers{mag: magazine.Magazine{}, godmode: newTestGodMode()}.WithInventory(inv)

	r := httptest.NewRequest("GET", "/auth?account=oci/frame", nil)
	r.SetBasicAuth("oci", "oci-pass")
	w := httptest.NewRecorder()
	h.AuthHandler(w, r) // fire-and-forget: must not fail the login
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp["token"])
}

func TestAuthHandler_OIDCEnvelopeCarriesResolvedSuffix(t *testing.T) {
	const signingKey = "test-envelope-signing-key-at-least-32-chars-long"
	fakeJWT := mintFakeAuthentikJWT(t, "alice", time.Now().Add(30*time.Minute), []byte("fake-inner-key"))

	hh, _, cleanup := newHandlerWithMockAuthentikAndKey(t, fakeJWT, signingKey)
	defer cleanup()

	st := newInventoryStub(t, resolveHit("frame"))
	hh = hh.WithInventory(st.client())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth?account=alice", nil)
	r.Header.Set("X-Forwarded-For", "192.168.5.115")
	r.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("alice:password")))
	hh.AuthHandler(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	token := resp["token"]
	require.NotEmpty(t, token)

	// The envelope must carry the RESOLVED rawUsername (alice/frame), not the
	// suffix-less original — the OCI middleware re-parses it unchanged.
	rawUsername, innerJWT, err := VerifyEnvelopeToken(token, []byte(signingKey))
	require.NoError(t, err)
	assert.Equal(t, "alice/frame", rawUsername)
	assert.Equal(t, fakeJWT, innerJWT)
	assert.Equal(t, 1, st.calls)

	// viper hygiene for the envelope key set by the shared helper.
	viper.Set("auth.envelope_signing_key", "")
}
