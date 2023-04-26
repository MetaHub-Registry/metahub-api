package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_Auth_AuthHandler_Success(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.AddBootstrap("users")
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs)
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

func Test_Auth_AuthHandler_Fail(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	_, err := SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs)
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

func Test_Auth_AuthMiddlewareHandler_WithUser(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.AddBootstrap("users")
	bs.SetEmptyAll()
	ctx := context.Background()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs)
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

func Test_Auth_AuthMiddlewareHandler_WithScope(t *testing.T) {
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
	defer TeardownTest(bs)
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

}
