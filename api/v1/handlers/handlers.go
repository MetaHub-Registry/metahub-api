package handlers

import (
	"net/http"

	"log"

	"github.com/metahub-registry/metahub-api/internal/data"
	"github.com/metahub-registry/metahub-api/internal/driver"
)

type Handlers struct {
	environment string
	models      data.Models
}

func NewHandlers(env string, dbPool *driver.DB) Handlers {
	return Handlers{
		environment: env,
		models:      data.New(dbPool.SQL),
	}
}

// ClosureHandlerExample shows different ways you can pass dependencies into your handler
func (h Handlers) ClosureHandlerExample() http.HandlerFunc {
	log.Printf("ClosureHandlerExample")
	return func(rw http.ResponseWriter, r *http.Request) {
		log.Println("huhu")
		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte("Hello World"))

	}
}

// RegularHandlerExample implements a simple handler to use the RW and Request object
func (h Handlers) RegularHandlerExample(rw http.ResponseWriter, r *http.Request) {
	//handler logic
}

func (h Handlers) GetEnvironment() string {
	return h.environment
}
