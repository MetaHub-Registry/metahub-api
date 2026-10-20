package handlers

import (
	"context"
	"io"

	"github.com/metahub-registry/metahub-api/pkg/config"
	"github.com/metahub-registry/metahub-api/pkg/pullthrough"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"go.opentelemetry.io/otel"
)

var (
	tracer = otel.Tracer("metahub-api/handlers")
)

type Handlers struct {
	environment string
	mag         magazine.Magazine
	oidc        *OIDCProvider
	godmode     *GodModeAuth
	// registry holds the namespace-authz switches (add-registry-namespace-authz);
	// the zero value is behavior-neutral (both enforcement flags off).
	registry config.RegistryConfig
	// pullThrough is the async cache worker (add-pull-through-manifest-intercept).
	// Shared (pointer) across handler copies; nil when not wired (tests).
	pullThrough *pullthrough.Cache
	// inventory is the thin consumer of the metahub-admin-ui resolve endpoint
	// (openspec change hostname-profile-resolution). nil ⇒ disabled — the
	// AuthHandler call site checks nil and skips resolution entirely.
	inventory *InventoryClient
	// blobStat / upstreamBlobReader are test seams for the two pull-through code
	// paths that otherwise reach the concrete Minio backend / a live registry.
	// When nil they delegate to the magazine. See statBlob / openUpstreamBlob.
	blobStat           func(ctx context.Context, digest string) (size int64, exists bool, err error)
	upstreamBlobReader func(ctx context.Context, up upstream.UpstreamTriple, dig string) (io.ReadCloser, error)
}

// statBlob reports the size of digest in MetaHub storage (exists=false when
// absent). Uses the injected seam when present, else the magazine's Minio.
func (h Handlers) statBlob(ctx context.Context, digest string) (int64, bool, error) {
	if h.blobStat != nil {
		return h.blobStat(ctx, digest)
	}
	obj, err := h.mag.Minio.BlobStatObj(ctx, digest)
	if err != nil {
		return 0, false, err
	}
	return obj.Size, true, nil
}

// openUpstreamBlob opens the upstream reader for a layer digest. Uses the
// injected seam when present, else the magazine's registry client.
func (h Handlers) openUpstreamBlob(ctx context.Context, up upstream.UpstreamTriple, dig string) (io.ReadCloser, error) {
	if h.upstreamBlobReader != nil {
		return h.upstreamBlobReader(ctx, up, dig)
	}
	rc, _, err := h.mag.GetBlobReaderByUpstreamTripleByDigest(ctx, up, dig)
	return rc, err
}

func NewHandlers(env string, mag magazine.Magazine, oidc *OIDCProvider, godmode *GodModeAuth) Handlers {
	return Handlers{
		environment: env,
		mag:         mag,
		oidc:        oidc,
		godmode:     godmode,
	}
}

// WithRegistry returns a copy of h with the registry namespace-authz config set.
// Server wiring calls this; tests that don't get the behavior-neutral zero value.
func (h Handlers) WithRegistry(rc config.RegistryConfig) Handlers {
	h.registry = rc
	return h
}

// WithPullThrough returns a copy of h with the async pull-through cache worker
// set. Server wiring calls this; tests that don't get the nil (inert) value.
func (h Handlers) WithPullThrough(c *pullthrough.Cache) Handlers {
	h.pullThrough = c
	return h
}

// WithInventory returns a copy of h with the inventory resolution client set
// (thin consumer of metahub-admin-ui's resolve surface). nil (or omitted) is
// the disabled state — behaviour-neutral, the call site checks nil.
func (h Handlers) WithInventory(c *InventoryClient) Handlers {
	h.inventory = c
	return h
}

// personalQuotaBytes is the default quota for a personal namespace, falling back
// to the built-in 5 GiB default when unset (e.g. in tests that enable switches
// without a full config).
func (h Handlers) personalQuotaBytes() int64 {
	if h.registry.DefaultUserQuotaBytes > 0 {
		return h.registry.DefaultUserQuotaBytes
	}
	return config.DefaultUserQuotaBytes
}

func (h Handlers) GetEnvironment() string {
	return h.environment
}
