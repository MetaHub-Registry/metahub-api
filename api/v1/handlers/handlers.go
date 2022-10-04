package handlers

import (
	"database/sql"

	"github.com/metahub-registry/metahub-api/pkg/backends/sqlite"
)

type Handlers struct {
	environment string
	models      sqlite.Models
}

func NewHandlers(env string, db *sql.DB) Handlers {
	models, err := sqlite.New(db)
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
