package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	gCtx "github.com/gorilla/context"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestAuthAuthHandlerSuccess(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.AddBootstrap("users")
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	/// Request
	token, err := br.Get("user:test#bearer")
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodHead, "/auth?account=qnib", nil)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	h.AuthHandler(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	var js map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &js)
	if err != nil {
		t.Fatal(err)
	}
	gotToken, ok := js["token"]
	assert.True(t, ok)
	assert.Equal(t, token, gotToken)
}

func TestAuthAuthHandlerFail(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	/// Request
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodHead, "/auth", nil)
	h.AuthHandler(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	var js map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &js)
	if err != nil {
		t.Fatal(err)
	}
	gotToken, ok := js["token"]
	assert.True(t, ok)
	assert.Equal(t, "REDACTED-IN-HISTORY", gotToken)
	// invalid Bearer
	r2 := httptest.NewRequest(http.MethodHead, "/auth?account=qnib", nil)
	r2.Header.Set("Authorization", "Bearer A B C")
	w2 := httptest.NewRecorder()
	h.AuthHandler(w2, r2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)
}
func TestAuthAuthMiddlewareHandlerWithUsers(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.AddBootstrap("users")
	bs.SetEmptyAll()
	bs.PostTestSleep = 0
	ctx := context.Background()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	for _, l := range br.ToLines() {
		if bs.PostTestSleep > 0 {
			t.Log(l)
		}
	}
	/// Request
	token, err := br.Get("user:admin#bearer")
	if err != nil {
		t.Fatal(err)
	}
	// Real user
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodHead, "/v2", nil)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	h.AuthMiddlewareHandler(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	gCtxUser := gCtx.Get(r, "userName")
	assert.Equal(t, "admin", gCtxUser)
	gCtxUserRole := gCtx.Get(r, "userRole")
	assert.Equal(t, "admin", gCtxUserRole)
}

func TestAuthAuthMiddlewareHandlerWithUser(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.AddBootstrap("users")
	bs.SetEmptyAll()
	bs.PostTestSleep = 0
	ctx := context.Background()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	for _, l := range br.ToLines() {
		if bs.PostTestSleep > 0 {
			t.Log(l)
		}
	}
	/// Request
	token, err := br.Get("user:test#bearer")
	if err != nil {
		t.Fatal(err)
	}
	// Real user
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodHead, "/v2", nil)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	h.AuthMiddlewareHandler(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	gCtxUser := gCtx.Get(r, "userName")
	assert.Equal(t, "test", gCtxUser)
	gCtxUserRole := gCtx.Get(r, "userRole")
	assert.Equal(t, "user", gCtxUserRole)
	// No auth
	w0 := httptest.NewRecorder()
	r0 := httptest.NewRequest(http.MethodHead, "/v2", nil)
	h.AuthMiddlewareHandler(w0, r0)
	assert.Equal(t, http.StatusUnauthorized, w0.Code)

	// Empty auth
	w1 := httptest.NewRecorder()
	r1 := httptest.NewRequest(http.MethodHead, "/v2", nil)
	r1.Header.Set("Authorization", "")
	h.AuthMiddlewareHandler(w1, r1)
	assert.Equal(t, http.StatusUnauthorized, w1.Code)
	// invalid Bearer
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodHead, "/v2", nil)
	r2.Header.Set("Authorization", "Bearer A B C")
	h.AuthMiddlewareHandler(w2, r2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)
}

func TestAuthAuthMiddlewareHandlerWithScope(t *testing.T) {
	testRepo := "test/image"
	testTag := "tag123"
	bs := utils.NewBootstrap()
	bs.AddBootstrap("users")
	//bs.SetSqlite("Desktop/metahub.db")
	bs.SetEmptyAll()
	ctx := context.Background()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	/// Request
	token, err := br.Get("user:test#bearer")
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	url := fmt.Sprintf("/v2/%s/manifests/%s", testRepo, testTag)
	r := httptest.NewRequest(http.MethodPut, url, nil)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	h.AuthMiddlewareHandler(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	token, err = br.Get("user:admin#bearer")
	if err != nil {
		t.Fatal(err)
	}
	wAdmin := httptest.NewRecorder()

	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	h.AuthMiddlewareHandler(wAdmin, r)
	assert.Equal(t, http.StatusOK, wAdmin.Code)
	h.AuthMiddlewareFunc(wAdmin, r)
	deb := gCtx.Get(r, "debug")
	assert.Equal(t, 0, deb)

}

// ---------------------------------------------------------------------------
// realmForRequest tests
// ---------------------------------------------------------------------------

func TestRealmForRequest(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		proto    string
		expected string
	}{
		{"metahub-registry.org host", "metahub-registry.org", "HTTP/1.1", "https://metahub-registry.org"},
		{"metahub.qnib.org host", "metahub.qnib.org", "HTTP/1.1", "https://metahub.qnib.org"},
		{"HTTP/2.0 unknown host", "custom.example.com", "HTTP/2.0", "https://custom.example.com"},
		{"HTTP/1.1 unknown host", "custom.example.com", "HTTP/1.1", "http://custom.example.com"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Host = tc.host
			r.Proto = tc.proto
			got := realmForRequest(r)
			assert.Equal(t, tc.expected, got)
		})
	}
}

// ---------------------------------------------------------------------------
// parseTokenParts tests
// ---------------------------------------------------------------------------

func TestParseTokenParts_MissingHeader(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v2", nil)
	req := types.Request{Repo: "test/repo", Method: http.MethodGet}
	_, _, ok := parseTokenParts(w, r, "http://example.com", req)
	assert.False(t, ok)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestParseTokenParts_InvalidHeader(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v2", nil)
	r.Header.Set("Authorization", "Bearer A B C")
	req := types.Request{Repo: "test/repo", Method: http.MethodGet}
	_, _, ok := parseTokenParts(w, r, "http://example.com", req)
	assert.False(t, ok)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestParseTokenParts_InvalidHeader_EmptyRepo(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v2", nil)
	r.Header.Set("Authorization", "Bearer A B C")
	req := types.Request{Repo: "", Method: http.MethodGet}
	_, _, ok := parseTokenParts(w, r, "http://example.com", req)
	assert.False(t, ok)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestParseTokenParts_ValidHeader(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v2", nil)
	r.Header.Set("Authorization", "Bearer mytoken123")
	req := types.Request{Repo: "test/repo", Method: http.MethodGet}
	accessToken, _, ok := parseTokenParts(w, r, "http://example.com", req)
	assert.True(t, ok)
	assert.Equal(t, "mytoken123", accessToken)
}

func TestAuthMiddleware(t *testing.T) {
	expected := `{"token":"REDACTED-IN-HISTORY"}`
	req := httptest.NewRequest(http.MethodGet, "/greet?name=john", nil)
	w := httptest.NewRecorder()
	h.AuthHandler(w, req)
	res := w.Result()
	defer res.Body.Close()
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Errorf("Error: %v", err)
	}

	assert.JSONEq(t, expected, string(data))
}

//
