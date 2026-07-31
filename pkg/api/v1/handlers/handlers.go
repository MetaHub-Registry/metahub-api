package handlers

import (
	"github.com/metahub-registry/metahub-api/pkg/config"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
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
