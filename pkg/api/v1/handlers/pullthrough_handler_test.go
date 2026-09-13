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