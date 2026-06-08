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

const (
	apiTimeout                = 5 * time.Second
	ImgBlobUploadUuidPath     = "/{image}/blobs/uploads/{uuid}"
	RepoImgBlobUploadUuidPath = "/{repo}/{image}/blobs/uploads/{uuid}"
	imgManifestRefPath        = "/{image}/manifests/{reference}"
	repoImgManifestRefPath    = "/{repo}/{image}/manifests/{reference}"
	imgBlobRefPath            = "/{image}/blobs/{reference}"
	repoImgBlobRefPath        = "/{repo}/{image}/blobs/{reference}"
)

func Initialize(mag magazine.Magazine, oidcProvider *handlers.OIDCProvider, godmode *handlers.GodModeAuth) *chi.Mux {
	// Handlers
	h := handlers.NewHandlers("dev", mag, oidcProvider, godmode)
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
		r.Post("/collections", h.RepoGetAll)
		r.Post("/collection", h.RepoGetOne)
		// ORAS artifact browsing (public)
		r.Get("/oras", h.OrasGetAll)
		r.Get("/oras/artifact", h.OrasGetOne)
		// Profiles (public)
		r.Get("/profiles/{label}", h.ProfileGetOne)
		r.Get("/profiles", h.ProfilesGetAll)
		// Upstream registries (public)
		r.Get("/registries", h.RegistriesGetAll)
		// Build-info (public) — reports the metahub-api + metalib
		// versions baked into the running binary. Used by the GoCD
		// e2e-dev verify-ce guard to confirm a fresh deploy landed
		// before running behaviour-dependent tests.
		r.Get("/version", h.VersionGetOne)
		// OIDC login / callback (public)
		r.Get("/login", h.OIDCLogin)
		r.Get("/callback", h.OIDCCallback)
		// Auth mode discovery (public) — lets the UI decide which login flow to render.
		r.Get("/auth/config", h.AuthConfig)
		// God-mode JSON login endpoint (public) — returns 404 when god-mode is disabled.
		r.Post("/godmode/login", h.GodModeLogin)
	})
	router.Route("/api/v1/protected", func(r chi.Router) {
		r.Use(middleware.Timeout(apiTimeout))
		r.Use(h.OIDCAuthMiddleware())
		r.Get("/logout", h.OIDCLogout)
		r.Get("/session", h.UserGetSession)
		// Registry token management (Authentik app passwords)
		r.Post("/registry-token", h.RegistryTokenCreate)
		r.Get("/registry-tokens", h.RegistryTokenList)
		r.Delete("/registry-tokens/{identifier}", h.RegistryTokenRevoke)
		r.Get("/user", h.UserGetOne)
		r.Get("/users/{id}", h.UserGetOne)
		r.Put("/users/{id}/secrets", h.UserPutSecret)
		r.Put("/user/secret", h.UserPutSecret)
		r.Delete("/user/secret/{id}", h.UserDelSecret)
	})
	router.Get("/auth", h.AuthHandler)
	/////// Subroute with AuthMiddleware to trigger login
	// (even if the token is the anonymous one)
	router.Route("/v2", func(r chi.Router) {
		if mag.Config.Tracing.Enabled {
			logrus.Info("Tracing enabled")
			r.Use(otelchi.Middleware("metahub-api", otelchi.WithChiRoutes(r)))
		}
		// All /v2/ routes (including heartbeat) go through auth middleware.
		// Docker uses GET /v2/ to discover the auth scheme — it must return
		// 401 with WWW-Authenticate when no valid token is present.
		r.Group(func(r chi.Router) {
			r.Use(h.OCIAuthMiddleware())
			r.Get("/", h.OCIHeartbeatHandler)
			// TODO: Is there no way to match multiple lengh path?
			r.Head(imgManifestRefPath, h.ManifestHeadHandler)
			r.Get(imgManifestRefPath, h.ManifestGetHandler)
			r.Head(repoImgManifestRefPath, h.ManifestHeadHandler)
			r.Get(repoImgManifestRefPath, h.ManifestGetHandler)
			r.Head("/{repo}/{repo2}/{image}/manifests/{reference}", h.ManifestHeadHandler)
			r.Get("/{repo}/{repo2}/{image}/manifests/{reference}", h.ManifestGetHandler)
			r.Head("/{repo}/{repo2}/{repo3}/{image}/manifests/{reference}", h.ManifestHeadHandler)
			r.Get("/{repo}/{repo2}/{repo3}/{image}/manifests/{reference}", h.ManifestGetHandler)

			//// Upload Manifest
			r.Put(imgManifestRefPath, h.ManifestPutHandler)
			r.Put(repoImgManifestRefPath, h.ManifestPutHandler)
			//// Delete
			r.Delete(imgManifestRefPath, h.ManifestDelHandler)
			r.Delete(repoImgManifestRefPath, h.ManifestDelHandler)
			// Blobs
			r.Get(imgBlobRefPath, h.BlobHandler)
			r.Get(repoImgBlobRefPath, h.BlobHandler)
			r.Get("/{repo}/{repo2}/{image}/blobs/{reference}", h.BlobHandler)
			r.Get("/{repo}/{repo2}/{repo3}/{image}/blobs/{reference}", h.BlobHandler)
			///// HEAD is the same w/o actually writing a body
			r.Head(imgBlobRefPath, h.BlobHandler)
			r.Head(repoImgBlobRefPath, h.BlobHandler)
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
			r.Delete(repoImgBlobRefPath, h.BlobDelHandler)
			r.Delete(imgBlobRefPath, h.BlobDelHandler)
			// Referrers
			r.Get("/{image}/referrers/{reference}", h.ReferrersGetHandler)
			r.Get("/{repo}/{image}/referrers/{reference}", h.ReferrersGetHandler)
			r.Get("/{repo}/{repo2}/{image}/referrers/{reference}", h.ReferrersGetHandler)
			r.Get("/{repo}/{repo2}/{repo3}/{image}/referrers/{reference}", h.ReferrersGetHandler)
		})
	})
	router.Get("/", h.RedirectToWWW)
	return router
}
