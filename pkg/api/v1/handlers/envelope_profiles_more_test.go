package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func TestExpFromJWT(t *testing.T) {
	// valid JWT with an exp claim -> parsed time
	exp := time.Now().Add(time.Hour).Truncate(time.Second)
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"exp": exp.Unix()})
	signed, err := tok.SignedString([]byte("k"))
	require.NoError(t, err)
	got, err := expFromJWT(signed)
	require.NoError(t, err)
	assert.Equal(t, exp.Unix(), got.Unix())

	// not a JWT -> error
	_, err = expFromJWT("not-a-jwt")
	assert.Error(t, err)

	// JWT without an exp claim -> error
	noExp := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "x"})
	signedNoExp, err := noExp.SignedString([]byte("k"))
	require.NoError(t, err)
	_, err = expFromJWT(signedNoExp)
	assert.Error(t, err)
}

// fakeProfilesDB implements ProfileGetAll for the profiles listing handler.
type fakeProfilesDB struct {
	magazine.Database
	profiles []types.Profile
	err      error
}

func (f *fakeProfilesDB) ProfileGetAll(context.Context) ([]types.Profile, error) {
	return f.profiles, f.err
}

func TestProfilesGetAll(t *testing.T) {
	// success
	h := Handlers{mag: magazine.Magazine{Database: &fakeProfilesDB{profiles: []types.Profile{{Label: "cpu/zen2"}}}}}
	w := httptest.NewRecorder()
	h.ProfilesGetAll(w, httptest.NewRequest("GET", "/profiles", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "profiles")

	// backend error -> 500
	h = Handlers{mag: magazine.Magazine{Database: &fakeProfilesDB{err: context.DeadlineExceeded}}}
	w = httptest.NewRecorder()
	h.ProfilesGetAll(w, httptest.NewRequest("GET", "/profiles", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestProfileGetOne_MissingLabel(t *testing.T) {
	// no {label} route param -> 400
	h := Handlers{mag: magazine.Magazine{Database: &fakeProfilesDB{}}}
	w := httptest.NewRecorder()
	h.ProfileGetOne(w, httptest.NewRequest("GET", "/profiles/", nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
