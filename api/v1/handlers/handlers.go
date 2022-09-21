package handlers

import (
	"github.com/metahub-registry/metahub-api/internal/data"
	"github.com/metahub-registry/metahub-api/internal/driver"
	"github.com/metahub-registry/metahub-api/pkg/config"
)

type Handlers struct {
	environment string
	models      data.Models
}

func NewHandlers(env string, dbPool *driver.DB) Handlers {
	ncfg := config.NeoConfig{
		DbName: "neo4j",
		User:   "neo4j",
		Passwd: "test",
		URL:    "bolt://localhost:7687",
	}
	models, err := data.New(dbPool.SQL, ncfg)
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
