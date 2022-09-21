package router

import (
	"time"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"
	"github.com/metahub-registry/metahub-api/api/v1/handlers"
	"github.com/metahub-registry/metahub-api/internal/driver"
)

const (
	envKeyJaegerEndpointURL = "JAEGER_ENDPOINT_URL"
	addr                    = ":8090"
	serviceName             = "metahub-api"
)

//var tracer oteltrace.Tracer

func Initialize(db *driver.DB) *chi.Mux {
	/*var err error
	tracer, err = utils.NewTracer(serviceName, os.Getenv(envKeyJaegerEndpointURL))
	if err != nil {
		log.Fatalf("unable to initialize tracer due: %v", err)
	}
	*/
	// Handlers
	h := handlers.NewHandlers("dev", db)
	// define router
	router := chi.NewRouter()

	router.Use(
		render.SetContentType(render.ContentTypeJSON), //forces Content-type
		middleware.RedirectSlashes,
		middleware.Recoverer,            //middleware to recover from panics
		middleware.Heartbeat("/health"), //for heartbeat process such as Kubernetes liveprobeness
		//otelchi.Middleware(serviceName, otelchi.WithChiRoutes(router)), //for tracing
		cors.Handler(cors.Options{
			// AllowedOrigins:   []string{"https://foo.com"}, // Use this to allow specific origin hosts
			AllowedOrigins: []string{"https://*", "http://*"},
			// AllowOriginFunc:  func(r *http.Request, origin string) bool { return true },
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
			ExposedHeaders:   []string{"Link"},
			AllowCredentials: false,
			MaxAge:           300, // Maximum value not ignored by any of major browsers
		}),
	)

	//Sets context for all requests
	router.Use(middleware.Timeout(30 * time.Second))
	router.Post("/users/login", h.Login)
	router.Post("/users/logout", h.Logout)
	router.Post("/users/signup", h.Signup)
	router.Route("/v1", func(r chi.Router) {
		r.Post("/collections", h.CollectionGetAll)
		r.Post("/collection", h.CollectionGetOne)
		//r.Mount("/metrics", nil)        //for monitoring agents such as prometheus
	})

	return router
}
