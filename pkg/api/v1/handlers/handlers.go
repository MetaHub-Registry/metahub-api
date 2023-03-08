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
}

func NewHandlers(env string, mag magazine.Magazine) Handlers {
	return Handlers{
		environment: env,
		mag:         mag,
	}
}

func (h Handlers) GetEnvironment() string {
	return h.environment
}
