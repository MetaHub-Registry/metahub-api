package handlers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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

// deliveryHandler builds a pull-through handler with a redirect-mode policy, a
// fake DB (upstream edge present/absent) and injectable blob-stat / upstream
// reader seams. The returned counter counts upstream reader opens.
func deliveryHandler(redirect, upFound bool) (Handlers, *int) {
	reg := magazine.RegistryConfig{
		Name: "harbor.example.com", URL: "harbor.example.com", TLS: false, AllowAnon: true,
		PullThrough: magazine.PullThroughConfig{
			Enabled: true, Cache: "async", ClientRedirect: redirect, RefreshInterval: time.Hour,
		},
	}
	mag := magazine.Magazine{
		Config:   magazine.MagConfig{Registries: []magazine.RegistryConfig{reg}},
		Database: ptFakeDB{up: mhtypes.Upstream{Reg: "harbor.example.com", Repo: "library/alpine", Digest: "sha256:deadbeef"}, upFound: upFound},
	}
	h := NewHandlers("test", mag, nil, nil).
		WithPullThrough(pullthrough.New(nil, pullthrough.MagazinePolicies{Mag: mag}, nil, pullthrough.Options{QueueSize: 8}))
	h.blobStat = func(_ context.Context, _ string) (int64, bool, error) {
		return 0, false, errors.New("object absent")
	}
	opens := 0
	h.upstreamBlobReader = func(_ context.Context, _ upstream.UpstreamTriple, _ string) (io.ReadCloser, error) {
		opens++
		return io.NopCloser(bytes.NewReader([]byte("layer-bytes"))), nil
	}
	return h, &opens
}

func blobReq(repo string) mhtypes.Request {
	return mhtypes.Request{Repo: repo, Digest: "sha256:deadbeef", Tag: "3.24", Profile: "generic"}
}

// TestResolveBlobOnMiss_LocalHit: bytes present → synthesised descriptor, no
// upstream routing (task 6.2 / blob-already-local).
func TestResolveBlobOnMiss_LocalHit(t *testing.T) {
	h, opens := deliveryHandler(true, true)
	h.blobStat = func(_ context.Context, _ string) (int64, bool, error) { return 123, true, nil }

	ctx := context.Background()
	w := httptest.NewRecorder()
	blob, stop := h.resolveBlobOnMiss(ctx, w, httptest.NewRequest(http.MethodGet, "/v2/x", nil),
		logrus.NewEntry(logrus.New()), trace.SpanFromContext(ctx), blobReq("harbor.example.com/library/alpine"))

	require.False(t, stop)
	require.NotNil(t, blob)
	assert.Equal(t, int64(123), blob.Size)
	assert.Equal(t, 0, *opens, "a local hit must not touch upstream")
}

// TestResolveBlobOnMiss_Redirect: absent bytes + client_redirect → 307 upstream.
func TestResolveBlobOnMiss_Redirect(t *testing.T) {
	h, _ := deliveryHandler(true, true)
	ctx := context.Background()
	w := httptest.NewRecorder()
	_, stop := h.resolveBlobOnMiss(ctx, w, httptest.NewRequest(http.MethodGet, "/v2/x", nil),
		logrus.NewEntry(logrus.New()), trace.SpanFromContext(ctx), blobReq("harbor.example.com/library/alpine"))

	require.True(t, stop)
	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "harbor.example.com")
}

// TestResolveBlobOnMiss_Proxy: absent bytes + no client_redirect → proxy the
// upstream bytes; a subsequent (now-cached) miss is served locally with no
// further upstream open (tasks 6.1/6.2/6.3).
func TestResolveBlobOnMiss_Proxy(t *testing.T) {
	h, opens := deliveryHandler(false, true)
	ctx := context.Background()
	w := httptest.NewRecorder()
	_, stop := h.resolveBlobOnMiss(ctx, w, httptest.NewRequest(http.MethodGet, "/v2/x", nil),
		logrus.NewEntry(logrus.New()), trace.SpanFromContext(ctx), blobReq("harbor.example.com/library/alpine"))

	require.True(t, stop)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "layer-bytes", w.Body.String())
	assert.Equal(t, 1, *opens, "first pull proxies upstream once")

	// Second pull: bytes now cached locally → served without upstream.
	h.blobStat = func(_ context.Context, _ string) (int64, bool, error) { return 11, true, nil }
	w2 := httptest.NewRecorder()
	blob, stop2 := h.resolveBlobOnMiss(ctx, w2, httptest.NewRequest(http.MethodGet, "/v2/x", nil),
		logrus.NewEntry(logrus.New()), trace.SpanFromContext(ctx), blobReq("harbor.example.com/library/alpine"))
	require.False(t, stop2)
	require.NotNil(t, blob)
	assert.Equal(t, 1, *opens, "second pull must not touch upstream")
}

// TestResolveBlobOnMiss_NonAlias404: absent bytes and no pull-through upstream →
// the existing not-found path.
func TestResolveBlobOnMiss_NonAlias404(t *testing.T) {
	h, _ := deliveryHandler(true, true)
	ctx := context.Background()
	w := httptest.NewRecorder()
	_, stop := h.resolveBlobOnMiss(ctx, w, httptest.NewRequest(http.MethodGet, "/v2/x", nil),
		logrus.NewEntry(logrus.New()), trace.SpanFromContext(ctx), blobReq("qnib/local"))

	require.True(t, stop)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestResolveBlobOnMiss_UpstreamGone404: an alias with no upstream edge → 404.
func TestResolveBlobOnMiss_UpstreamGone404(t *testing.T) {
	h, _ := deliveryHandler(true, false)
	ctx := context.Background()
	w := httptest.NewRecorder()
	_, stop := h.resolveBlobOnMiss(ctx, w, httptest.NewRequest(http.MethodGet, "/v2/x", nil),
		logrus.NewEntry(logrus.New()), trace.SpanFromContext(ctx), blobReq("harbor.example.com/library/alpine"))

	require.True(t, stop)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
