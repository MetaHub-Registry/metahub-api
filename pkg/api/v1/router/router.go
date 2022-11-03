package router

import (
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"
	"github.com/metahub-registry/metahub-api/pkg/api/v1/handlers"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
)

func Initialize(mag magazine.Magazine) *chi.Mux {
	// Handlers
	h := handlers.NewHandlers("dev", mag)
	// define router
	router := chi.NewRouter()

	router.Use(
		render.SetContentType(render.ContentTypeJSON), //forces Content-type
		middleware.StripSlashes,
		middleware.Logger,
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
		r.Use(h.AuthMiddleware)
		r.Get("/", h.HeartbeatHandler)
		// TODO: Is there no way to match multiple lengh path?
		r.Head("/{image}/manifests/{reference}", h.ManifestHeadHandler)
		r.Get("/{image}/manifests/{reference}", h.ManifestGetHandler)
		r.Head("/{repo}/{image}/manifests/{reference}", h.ManifestHeadHandler)
		r.Get("/{repo}/{image}/manifests/{reference}", h.ManifestGetHandler)
		//// Upload Manifest
		r.Put("/{image}/manifests/{reference}", h.ManifestPutHandler)
		r.Put("/{repo}/{image}/manifests/{reference}", h.ManifestPutHandler)
		// Blobs
		r.Get("/{image}/blobs/{reference}", h.BlobHandler)
		r.Get("/{repo}/{image}/blobs/{reference}", h.BlobHandler)
		///// HEAD is the same w/o actually writing a body
		r.Head("/{image}/blobs/{reference}", h.BlobHandler)
		r.Head("/{repo}/{image}/blobs/{reference}", h.BlobHandler)

		// Tags
		r.Get("/{image}/tags/list", h.TagGetHandler)
		r.Get("/{repo}/{image}/tags/list", h.TagGetHandler)
		r.Get("/_catalog", h.CatalogGetHandler)
		// Upload Blobs
		r.Post("/{repo}/{image}/blobs/uploads", h.BlobUploadPostHandler)
		r.Post("/{image}/blobs/uploads", h.BlobUploadPostHandler)
		r.Patch("/{image}/blobs/uploads/{uuid}", h.BlobUploadPatchHandler)
		r.Patch("/{repo}/{image}/blobs/uploads/{uuid}", h.BlobUploadPatchHandler)
		r.Put("/{image}/blobs/uploads/{uuid}", h.BlobUploadPutHandler)
		r.Put("/{repo}/{image}/blobs/uploads/{uuid}", h.BlobUploadPutHandler)
	})
	return router
}
