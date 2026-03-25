package handlers

import (
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
}

func NewHandlers(env string, mag magazine.Magazine, oidc *OIDCProvider, godmode *GodModeAuth) Handlers {
	return Handlers{
		environment: env,
		mag:         mag,
		oidc:        oidc,
		godmode:     godmode,
	}
}

func (h Handlers) GetEnvironment() string {
	return h.environment
}
