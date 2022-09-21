package handlers

import (
	"github.com/metahub-registry/metahub-api/internal/data"
	"github.com/metahub-registry/metahub-api/pkg/config"
)

type Handlers struct {
	environment string
	models      data.Models
}

func NewHandlers(env string, ncfg config.NeoConfig) Handlers {
	models, err := data.New(ncfg)
	if err != nil {
		panic(err)
	}
	return Handlers{
		environment: env,
		models:      models,
	}
}

func (h Handlers) GetEnvironment() string {
	return h.environment
}
