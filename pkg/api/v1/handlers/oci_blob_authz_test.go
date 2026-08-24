package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-api/pkg/config"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

// fakePushAuthzDB records whether the legacy push rule was consulted at all,
// which is the property under test -- not merely what it returned.
type fakePushAuthzDB struct {
	magazine.Database
	called bool
	err    error
}

func (f *fakePushAuthzDB) ProfileAuthorizedPushHandler(_ context.Context, _ types.Request) (types.Profile, error) {
	f.called = true
	return types.Profile{}, f.err
}

// TestLegacyPushAuthz guards the blob-path half of the namespace-authz switch.
//
// ProfileAuthorizedPushHandler only lets a non-admin push to a repo whose path
// starts with their own username. Every blob handler used to call it
// unconditionally, so with enforce_write_ownership on, a project admin already
// authorised by OCIWriteAuthMiddleware via mh-<org>-<project>-admin was refused
// at the first blob upload -- the namespace matrix could never grant a
// non-admin a push of new content.
func TestLegacyPushAuthz(t *testing.T) {
	req := types.Request{Repo: "acme/eng/app", User: "alice", Profile: "generic"}
	denied := func() error { return errors.New("is not allowed to push to repo") }

	t.Run("enforcement on skips the legacy rule entirely", func(t *testing.T) {
		db := &fakePushAuthzDB{err: denied()}
		h := Handlers{mag: magazine.Magazine{Database: db}}.
			WithRegistry(config.RegistryConfig{EnforceWriteOwnership: true})

		require.NoError(t, h.legacyPushAuthz(context.Background(), req),
			"namespace matrix is authoritative, legacy rule must not deny")
		assert.False(t, db.called,
			"legacy rule must not even be consulted when ownership enforcement is on")
	})

	t.Run("enforcement off preserves the legacy denial", func(t *testing.T) {
		db := &fakePushAuthzDB{err: denied()}
		h := Handlers{mag: magazine.Magazine{Database: db}}

		assert.Error(t, h.legacyPushAuthz(context.Background(), req))
		assert.True(t, db.called)
	})

	t.Run("enforcement off passes when the legacy rule permits", func(t *testing.T) {
		db := &fakePushAuthzDB{}
		h := Handlers{mag: magazine.Magazine{Database: db}}

		assert.NoError(t, h.legacyPushAuthz(context.Background(), req))
		assert.True(t, db.called)
	})
}

// TestBlobHandlersHonourLegacyDenial checks each blob handler still refuses,
// and refuses EARLY, when the legacy rule denies and enforcement is off -- the
// behaviour that must survive the switch being introduced.
//
// Only the enforcement-off case is driven through the handlers: with it on they
// would carry on past the gate into the database, which is a bare interface
// here. The skip itself is asserted directly on the helper above.
func TestBlobHandlersHonourLegacyDenial(t *testing.T) {
	handlers := map[string]func(Handlers) http.HandlerFunc{
		"BlobUploadPutHandler":   func(h Handlers) http.HandlerFunc { return h.BlobUploadPutHandler },
		"BlobUploadPatchHandler": func(h Handlers) http.HandlerFunc { return h.BlobUploadPatchHandler },
		"BlobUploadPostHandler":  func(h Handlers) http.HandlerFunc { return h.BlobUploadPostHandler },
		"BlobDelHandler":         func(h Handlers) http.HandlerFunc { return h.BlobDelHandler },
	}
	for name, pick := range handlers {
		t.Run(name, func(t *testing.T) {
			db := &fakePushAuthzDB{err: errors.New("is not allowed to push to repo")}
			h := Handlers{mag: magazine.Magazine{Database: db}} // enforcement OFF

			w := httptest.NewRecorder()
			pick(h)(w, httptest.NewRequest(http.MethodPost, "/v2/acme/eng/app/blobs/uploads/", nil))

			assert.True(t, db.called, "legacy rule must run when enforcement is off")
			assert.GreaterOrEqual(t, w.Code, http.StatusBadRequest,
				"a legacy denial must short-circuit the handler")
		})
	}
}
