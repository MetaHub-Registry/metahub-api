package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	"github.com/metahub-registry/metahub-api/pkg/pullthrough"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
	mhtypes "github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
)

// ptFakeDB embeds the magazine.Database interface (nil) and overrides only the
// two methods the pull-through routing consults, so the handlers can be
// exercised without Neo4j/MinIO/registry.
type ptFakeDB struct {
	magazine.Database
	up        mhtypes.Upstream
	upFound   bool
	last      int64
	lastFound bool
}

func (f ptFakeDB) UpstreamGetByLayerDigest(_ context.Context, _ string) (mhtypes.Upstream, bool, error) {
	return f.up, f.upFound, nil
}

func (f ptFakeDB) CollectionTagGetLastRefreshed(_ context.Context, _, _ string) (int64, bool, error) {
	return f.last, f.lastFound, nil
}

func ptMagazine(db magazine.Database) magazine.Magazine {
	return magazine.Magazine{
		Config: magazine.MagConfig{Registries: []magazine.RegistryConfig{{
			Name:      "harbor.example.com",
			URL:       "harbor.example.com",
			TLS:       false,
			AllowAnon: true,
			PullThrough: magazine.PullThroughConfig{
				Enabled:         true,
				Cache:           "async",
				ClientRedirect:  true,
				RefreshInterval: time.Hour,
			},
		}}},
		Database: db,
	}
}

func ptReq() mhtypes.Request {
	return mhtypes.Request{Repo: "harbor.example.com/library/alpine", Digest: "sha256:deadbeef", Tag: "3.24", Profile: "generic"}
}

// TestTryPullThroughBlobMiss_Guards covers the early-return branches: nil cache,
// non-GET method, and a repo that does not resolve to a configured upstream.
func TestTryPullThroughBlobMiss_Guards(t *testing.T) {
	ctx := context.Background()
	l := logrus.NewEntry(logrus.New())
	span := trace.SpanFromContext(ctx)
	w := httptest.NewRecorder()

	if (Handlers{}).tryPullThroughBlobMiss(ctx, w, httptest.NewRequest(http.MethodGet, "/v2/x", nil), l, span, ptReq()) {
		t.Fatal("nil pull-through cache must not route")
	}
	h := NewHandlers("test", ptMagazine(ptFakeDB{}), nil, nil).
		WithPullThrough(pullthrough.New(nil, pullthrough.MagazinePolicies{Mag: ptMagazine(ptFakeDB{})}, nil, pullthrough.Options{}))
	if h.tryPullThroughBlobMiss(ctx, w, httptest.NewRequest(http.MethodHead, "/v2/x", nil), l, span, ptReq()) {
		t.Fatal("HEAD must not route through pull-through")
	}
	nonAlias := ptReq()
	nonAlias.Repo = "qnib/local"
	if h.tryPullThroughBlobMiss(ctx, w, httptest.NewRequest(http.MethodGet, "/v2/x", nil), l, span, nonAlias) {
		t.Fatal("non-alias repo must not route through pull-through")
	}
}

// TestTryPullThroughBlobMiss_Redirect covers the client_redirect path: a blob
// miss under an enabled alias 307s to the upstream URL with Authorization
// stripped and enqueues a cache job.
func TestTryPullThroughBlobMiss_Redirect(t *testing.T) {
	db := ptFakeDB{upFound: true, up: mhtypes.Upstream{Reg: "harbor.example.com", Repo: "library/alpine", Digest: "sha256:deadbeef"}}
	mag := ptMagazine(db)
	h := NewHandlers("test", mag, nil, nil).
		WithPullThrough(pullthrough.New(nil, pullthrough.MagazinePolicies{Mag: mag}, nil, pullthrough.Options{QueueSize: 4}))

	ctx := context.Background()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v2/harbor.example.com/library/alpine/blobs/sha256:deadbeef", nil)
	r.Header.Set("Authorization", "Bearer secret")

	require.True(t, h.tryPullThroughBlobMiss(ctx, w, r, logrus.NewEntry(logrus.New()), trace.SpanFromContext(ctx), ptReq()))
	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "harbor.example.com")
}

// TestTryPullThroughBlobMiss_UpstreamNotFound covers the no-upstream-edge path.
func TestTryPullThroughBlobMiss_UpstreamNotFound(t *testing.T) {
	mag := ptMagazine(ptFakeDB{upFound: false})
	h := NewHandlers("test", mag, nil, nil).
		WithPullThrough(pullthrough.New(nil, pullthrough.MagazinePolicies{Mag: mag}, nil, pullthrough.Options{}))
	ctx := context.Background()
	w := httptest.NewRecorder()
	assert.False(t, h.tryPullThroughBlobMiss(ctx, w, httptest.NewRequest(http.MethodGet, "/v2/x", nil),
		logrus.NewEntry(logrus.New()), trace.SpanFromContext(ctx), ptReq()))
}

// redirectGateMagazine builds a magazine with a single pull-through registry
// whose AllowAnon/credentials/client_redirect settings are parameterized, so
// the redirect-gate matrix can be exercised.
func redirectGateMagazine(allowAnon, creds, clientRedirect, enabled bool) magazine.Magazine {
	rc := magazine.RegistryConfig{
		Name:      "harbor.example.com",
		URL:       "harbor.example.com",
		TLS:       false,
		AllowAnon: allowAnon,
		PullThrough: magazine.PullThroughConfig{
			Enabled:        enabled,
			Cache:          "async",
			ClientRedirect: clientRedirect,
		},
	}
	if creds {
		rc.User = "robot"
		rc.Password = "secret"
	}
	return magazine.Magazine{Config: magazine.MagConfig{Registries: []magazine.RegistryConfig{rc}}}
}

// gateHandlers wires a Handlers with the given magazine; the fake DB reports
// the layer's upstream as harbor.example.com so the redirect gate consults it.
func gateHandlers(mag magazine.Magazine) Handlers {
	return NewHandlers("test", mag, nil, nil).
		WithPullThrough(pullthrough.New(nil, pullthrough.MagazinePolicies{Mag: mag}, nil, pullthrough.Options{}))
}

var gateLyrUp = mhtypes.Upstream{Reg: "harbor.example.com", Repo: "library/alpine", Digest: "sha256:deadbeef"}

// TestUpstreamRedirectAllowed covers the Option-A gate matrix: pull-through
// registries are gated on client_redirect alone (proxy is the default for
// credentialed registries); legacy/unknown registries keep the AllowAnon gate.
func TestUpstreamRedirectAllowed(t *testing.T) {
	tests := []struct {
		name          string
		allowAnon     bool
		creds         bool
		clientRedir   bool
		enabled       bool
		want          bool
		wantLegacyAno bool // expect fallback decision when policy inactive
	}{
		{"credentialed + redirect off → proxy (A default)", true, true, false, true, false, false},
		{"anon + redirect off → proxy", true, false, false, true, false, false},
		{"credentialed private + redirect off → proxy", false, true, false, true, false, false},
		{"anon + redirect on → 307", true, false, true, true, true, false},
		{"credentialed private + redirect on → 307 (client holds upstream creds)", false, true, true, true, true, false},
		{"disabled pull-through, anon → legacy AllowAnon fallback", true, false, false, false, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := gateHandlers(redirectGateMagazine(tt.allowAnon, tt.creds, tt.clientRedir, tt.enabled))
			assert.Equal(t, tt.want, h.upstreamRedirectAllowed(gateLyrUp))
		})
	}
	// Unknown registry (no policy): legacy AllowAnon gate denies.
	h := gateHandlers(magazine.Magazine{Config: magazine.MagConfig{Registries: []magazine.RegistryConfig{}}})
	assert.False(t, h.upstreamRedirectAllowed(gateLyrUp), "unknown registry must not be redirect-eligible")
}

// TestTryBlobGetRedirect_ClientRedirectOn covers the full 307 path when the
// pull-through registry opts into client_redirect: the layer's upstream is
// known, the gate allows, and the redirect carries no Authorization header.
func TestTryBlobGetRedirect_ClientRedirectOn(t *testing.T) {
	db := ptFakeDB{upFound: true, up: gateLyrUp}
	mag := redirectGateMagazine(true, true, true, true) // credentialed, opt-in
	h := gateHandlers(mag)
	// ptFakeDB must be reachable through the magazine for UpstreamGetByLayerDigest.
	h.mag.Database = db

	ctx := context.Background()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v2/harbor.example.com/library/alpine/blobs/sha256:deadbeef", nil)
	r.Header.Set("Authorization", "Bearer metahub-token")

	require.True(t, h.tryBlobGetRedirect(ctx, w, r, logrus.NewEntry(logrus.New()), trace.SpanFromContext(ctx), ptReq(), upstream.UpstreamTriple{}))
	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "harbor.example.com")
	assert.Empty(t, w.Header().Get("Authorization"), "metahub token must be stripped")
	assert.NotContains(t, w.Header().Get("Location"), "robot", "upstream credentials must never leak into Location")
}

// countingRefresher records Refresh invocations.
type countingRefresher struct {
	mu sync.Mutex
	n  int
}

func (c *countingRefresher) Refresh(_ context.Context, _ pullthrough.RefreshJob) error {
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
	return nil
}

func (c *countingRefresher) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// TestMaybeRefreshManifest_EnqueuesWhenExpired covers the TTL path: an expired
// last_refreshed enqueues an async upstream re-check.
func TestMaybeRefreshManifest_EnqueuesWhenExpired(t *testing.T) {
	mag := ptMagazine(ptFakeDB{last: time.Now().Add(-2 * time.Hour).Unix(), lastFound: true})
	rf := &countingRefresher{}
	c := pullthrough.New(nil, pullthrough.MagazinePolicies{Mag: mag}, nil, pullthrough.Options{Workers: 1, QueueSize: 4}).
		WithRefresher(rf)
	ctx, cancel := context.WithCancel(context.Background())
	c.Start(ctx)
	defer func() { cancel(); c.Stop() }()

	h := NewHandlers("test", mag, nil, nil).WithPullThrough(c)
	h.maybeRefreshManifest(ctx, logrus.NewEntry(logrus.New()), trace.SpanFromContext(ctx),
		mhtypes.Request{Repo: "harbor.example.com/library/alpine", Tag: "3.24", Profile: "generic"})

	require.Eventually(t, func() bool { return rf.count() == 1 }, 2*time.Second, 10*time.Millisecond,
		"expired TTL must enqueue an async refresh")
}

// TestMaybeRefreshManifest_WithinTTLIsNoop covers the fresh-cache branch: a
// last_refreshed within the interval enqueues nothing.
func TestMaybeRefreshManifest_WithinTTLIsNoop(t *testing.T) {
	mag := ptMagazine(ptFakeDB{last: time.Now().Unix(), lastFound: true})
	rf := &countingRefresher{}
	c := pullthrough.New(nil, pullthrough.MagazinePolicies{Mag: mag}, nil, pullthrough.Options{Workers: 1, QueueSize: 4}).
		WithRefresher(rf)
	ctx, cancel := context.WithCancel(context.Background())
	c.Start(ctx)
	defer func() { cancel(); c.Stop() }()

	h := NewHandlers("test", mag, nil, nil).WithPullThrough(c)
	h.maybeRefreshManifest(ctx, logrus.NewEntry(logrus.New()), trace.SpanFromContext(ctx),
		mhtypes.Request{Repo: "harbor.example.com/library/alpine", Tag: "3.24", Profile: "generic"})

	time.Sleep(100 * time.Millisecond)
	require.Zero(t, rf.count(), "fresh cache must not enqueue a refresh")
}
// TestMaybeRefreshManifest_Guards covers the no-op guards.
func TestMaybeRefreshManifest_Guards(t *testing.T) {
	ctx := context.Background()
	l := logrus.NewEntry(logrus.New())
	span := trace.SpanFromContext(ctx)

	(Handlers{}).maybeRefreshManifest(ctx, l, span, ptReq())
	mag := ptMagazine(ptFakeDB{last: time.Now().Unix(), lastFound: true})
	h := NewHandlers("test", mag, nil, nil).
		WithPullThrough(pullthrough.New(nil, pullthrough.MagazinePolicies{Mag: mag}, nil, pullthrough.Options{}))
	digestReq := ptReq()
	digestReq.Digest = "sha256:x"
	h.maybeRefreshManifest(ctx, l, span, digestReq)
	nonAlias := ptReq()
	nonAlias.Repo = "qnib/local"
	h.maybeRefreshManifest(ctx, l, span, nonAlias)
}