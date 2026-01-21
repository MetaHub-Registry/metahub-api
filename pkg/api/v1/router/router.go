package router

import (
	"time"

	logger "github.com/chi-middleware/logrus-logger"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/riandyrn/otelchi"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/api/v1/handlers"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
)

const (
	apiTimeout                = 5 * time.Second
	ImgBlobUploadUuidPath     = "/{image}/blobs/uploads/{uuid}"
	RepoImgBlobUploadUuidPath = "/{repo}/{image}/blobs/uploads/{uuid}"
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
	)
	if mag.Config.CORS.Enabled {
		router.Use(cors.Handler(cors.Options{
			AllowedOrigins:   []string{"https://*", "http://*"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
			ExposedHeaders:   []string{"Link"},
			AllowCredentials: true,
			MaxAge:           300, // Maximum value not ignored by any of major browsers
		}))
	}

	//Sets context for all requests
	router.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.Timeout(apiTimeout))
		r.Post("/collections", h.CollectionGetAll)
		r.Post("/collection", h.CollectionGetOne)
		// Oauth2
		r.Get("/login", h.GitHubLogin)

	})
	router.Route("/api/v1/protected", func(r chi.Router) {
		r.Use(middleware.Timeout(apiTimeout))
		r.Use(jwtauth.Verifier(handlers.TokenAuth))
		r.Use(jwtauth.Authenticator)
		// Seek, verify and validate JWT tokens
		r.Get("/logout", h.GitHubLogout)
		r.Get("/session", h.UserGetSession)
		r.Get("/user", h.UserGetOne)
		r.Get("/users/{id}", h.UserGetOne)
		r.Put("/users/{id}/secrets", h.UserPutSecret)
		r.Put("/user/secret", h.UserPutSecret)
		r.Delete("/user/secret/{id}", h.UserDelSecret)
	})
	router.Get("/auth", h.AuthHandler)
	//router.Get("/logout", h.GitHubLogout)
	router.Get("/github_oauth_cb", h.GitHubCallback)
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
		r.Patch(ImgBlobUploadUuidPath, h.BlobUploadPatchHandler)
		r.Patch(RepoImgBlobUploadUuidPath, h.BlobUploadPatchHandler)
		r.Put(ImgBlobUploadUuidPath, h.BlobUploadPutHandler)
		r.Put(RepoImgBlobUploadUuidPath, h.BlobUploadPutHandler)
		r.Get(ImgBlobUploadUuidPath, h.BlobUploadGetHandler)
		r.Get(RepoImgBlobUploadUuidPath, h.BlobUploadGetHandler)
		// Delete Blobs
		r.Delete("/{repo}/{image}/blobs/{reference}", h.BlobDelHandler)
		r.Delete("/{image}/blobs/{reference}", h.BlobDelHandler)
		// Referrers
		r.Get("/{image}/referrers/{reference}", h.ReferrersGetHandler)
		r.Get("/{repo}/{image}/referrers/{reference}", h.ReferrersGetHandler)
		r.Get("/{repo}/{repo2}/{image}/referrers/{reference}", h.ReferrersGetHandler)
		r.Get("/{repo}/{repo2}/{repo3}/{image}/referrers/{reference}", h.ReferrersGetHandler)
	})
	router.Get("/", h.RedirectToWWW)
	return router
}
