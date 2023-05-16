package router

import (
	"time"

	logger "github.com/chi-middleware/logrus-logger"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"
	"github.com/riandyrn/otelchi"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/api/v1/handlers"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
)

func Initialize(mag magazine.Magazine) *chi.Mux {
	// Handlers
	h := handlers.NewHandlers("dev", mag)
	// define router
	router := chi.NewRouter()
	log := logrus.StandardLogger().WithFields(
		logrus.Fields{
			"packrusage": "pkg/api/v1/router/router.go",
			"package":    "router",
			"middleware": "logger",
		},
	)
	router.Use(
		render.SetContentType(render.ContentTypeJSON), //forces Content-type
		middleware.StripSlashes,
		logger.Logger("router", log),
		middleware.Recoverer,            //middleware to recover from panics
		middleware.Heartbeat("/health"), //for heartbeat process such as Kubernetes liveprobeness
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
	router.Route("/api/v1", func(r chi.Router) {
		r.Post("/collections", h.CollectionGetAll)
		r.Post("/collection", h.CollectionGetOne)
	})
	router.Get("/auth", h.AuthHandler)

	/////// Subroute with AuthMiddleware to trigger login
	// (even if the token is the anonymous one)
	router.Route("/v2", func(r chi.Router) {
		if mag.Config.Tracing.Enabled {
			logrus.Info("Tracing enabled")
			r.Use(otelchi.Middleware("metahub-api", otelchi.WithChiRoutes(r)))
		}
		r.Use(h.AuthMiddleware)
		r.Get("/", h.HeartbeatHandler)
		// TODO: Is there no way to match multiple lengh path?
		r.Head("/{image}/manifests/{reference}", h.ManifestHeadHandler)
		r.Get("/{image}/manifests/{reference}", h.ManifestGetHandler)
		r.Head("/{repo}/{image}/manifests/{reference}", h.ManifestHeadHandler)
		r.Get("/{repo}/{image}/manifests/{reference}", h.ManifestGetHandler)
		r.Head("/{repo}/{repo2}/{image}/manifests/{reference}", h.ManifestHeadHandler)
		r.Get("/{repo}/{repo2}/{image}/manifests/{reference}", h.ManifestGetHandler)
		r.Head("/{repo}/{repo2}/{repo3}/{image}/manifests/{reference}", h.ManifestHeadHandler)
		r.Get("/{repo}/{repo2}/{repo3}/{image}/manifests/{reference}", h.ManifestGetHandler)

		//// Upload Manifest
		r.Put("/{image}/manifests/{reference}", h.ManifestPutHandler)
		r.Put("/{repo}/{image}/manifests/{reference}", h.ManifestPutHandler)
		//// Delete
		r.Delete("/{image}/manifests/{reference}", h.ManifestDelHandler)
		r.Delete("/{repo}/{image}/manifests/{reference}", h.ManifestDelHandler)
		// Blobs
		r.Get("/{image}/blobs/{reference}", h.BlobHandler)
		r.Get("/{repo}/{image}/blobs/{reference}", h.BlobHandler)
		r.Get("/{repo}/{repo2}/{image}/blobs/{reference}", h.BlobHandler)
		r.Get("/{repo}/{repo2}/{repo3}/{image}/blobs/{reference}", h.BlobHandler)
		///// HEAD is the same w/o actually writing a body
		r.Head("/{image}/blobs/{reference}", h.BlobHandler)
		r.Head("/{repo}/{image}/blobs/{reference}", h.BlobHandler)
		r.Head("/{repo}/{repo2}/{image}/blobs/{reference}", h.BlobHandler)
		r.Head("/{repo}/{repo2}/{repo3}/{image}/blobs/{reference}", h.BlobHandler)

		// Tags
		r.Get("/{image}/tags/list", h.TagGetHandler)
		r.Get("/{repo}/{image}/tags/list", h.TagGetHandler)
		r.Get("/{repo}/{repo2}/{image}/tags/list", h.TagGetHandler)
		r.Get("/{repo}/{repo2}/{repo3}/{image}/tags/list", h.TagGetHandler)
		r.Get("/_catalog", h.CatalogGetHandler)
		// Upload Blobs
		r.Post("/{repo}/{image}/blobs/uploads", h.BlobUploadPostHandler)
		r.Post("/{image}/blobs/uploads", h.BlobUploadPostHandler)
		r.Patch("/{image}/blobs/uploads/{uuid}", h.BlobUploadPatchHandler)
		r.Patch("/{repo}/{image}/blobs/uploads/{uuid}", h.BlobUploadPatchHandler)
		r.Put("/{image}/blobs/uploads/{uuid}", h.BlobUploadPutHandler)
		r.Put("/{repo}/{image}/blobs/uploads/{uuid}", h.BlobUploadPutHandler)
		// Delete Blobs
		r.Delete("/{repo}/{image}/blobs/{reference}", h.BlobDelHandler)
		r.Delete("/{image}/blobs/{reference}", h.BlobDelHandler)
	})
	router.Get("/", h.RedirectToWWW)
	return router
}
