package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types/www"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestUserLogin(t *testing.T) {
	bs := utils.NewBootstrap()
	debug := 0
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	ctx := context.Background()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	err := h.mag.Minio.EmptyAll()
	if err != nil {
		t.Fatal(err)
	}
	br, err := SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {
		TeardownTest(bs)
		t.Fatal(err)
	}
	defer TeardownTest(bs)
	// Actual Tests
	adminPwd, err := br.Get("user:admin#password")
	if err != nil {
		t.Fatal(err)
	}
	// Login User
	if err := h.mag.UserManagement.LoginUser(ctx, "admin", adminPwd); err != nil {
		t.Fatal(err)
	}
	// FAILED LOGIN
	err = h.mag.UserManagement.LoginUser(ctx, "admin", "FAKE")
	if err == nil {
		t.Fatal("Should have failed")
	}
}

func TestUserGetOne(t *testing.T) {
	bs := utils.NewBootstrap()
	debug := 1
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	ctx := context.Background()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	err := h.mag.Minio.EmptyAll()
	if err != nil {
		t.Fatal(err)
	}
	br, err := SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {
		TeardownTest(bs)
		t.Fatal(err)
	}
	defer TeardownTest(bs)
	// Actual Tests
	sqlUsr, err := h.mag.UserManagement.GetByUsername(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	userID := fmt.Sprintf("%d", sqlUsr.ID)
	wGet0, rGet0 := utils.CreateNewHTTPTest(http.MethodGet,
		map[string]string{"id": userID}, map[string]string{},
		map[string]string{},
		"/", nil)
	h.UserGetOne(&wGet0, rGet0)
	if wGet0.Code != http.StatusOK {
		t.Fatalf("Should have returned 200, but got %d", wGet0.Code)
	}
	var resp www.User
	err = json.Unmarshal(wGet0.Body.Bytes(), &resp)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, sqlUsr.ID, resp.ID)
}

func TestUserCRUD(t *testing.T) {
	bs := utils.NewBootstrap()
	debug := 1
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	ctx := context.Background()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	err := h.mag.Minio.EmptyAll()
	if err != nil {
		t.Fatal(err)
	}
	br, err := SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {
		TeardownTest(bs)
		t.Fatal(err)
	}
	defer TeardownTest(bs)
	// Actual Tests
	wwwUser := www.User{
		GithubUsername: "test",
		Email:          "test@test.de",
		GithubUserID:   "123",
		Secrets:        []www.Secret{},
	}
	wwwBytes := wwwUser.ToBytes()
	w0, r0 := utils.CreateNewHTTPTest(http.MethodPost,
		map[string]string{}, map[string]string{},
		map[string]string{},
		"/users", wwwBytes)
	h.UserPost(&w0, r0)
	if w0.Code != http.StatusCreated {
		t.Fatalf("Should have returned 201, but got %d", w0.Code)
	}
	u0, err := h.mag.UserManagement.GetByUsername(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "123", u0.GithubID)
	///// GET /user/id
	userID := fmt.Sprintf("%d", u0.ID)
	wGet0, rGet0 := utils.CreateNewHTTPTest(http.MethodGet,
		map[string]string{"id": userID}, map[string]string{},
		map[string]string{},
		"/users/{id}", nil)
	h.UserGetOne(&wGet0, rGet0)
	if wGet0.Code != http.StatusOK {
		t.Fatalf("Should have returned 200, but got %d", wGet0.Code)
	}
	var resp www.User
	err = json.Unmarshal(wGet0.Body.Bytes(), &resp)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, u0.ID, resp.ID)
	////// PUT secret
	secret := www.Secret{
		Key:  "hash",
		Name: "test0",
	}
	wPut0, rPut0 := utils.CreateNewHTTPTest(http.MethodPut,
		map[string]string{"id": userID}, map[string]string{},
		map[string]string{},
		"/users/{id}/secrets", secret.ToBytes())
	h.UserPutSecret(&wPut0, rPut0)
	if wPut0.Code != http.StatusCreated {
		t.Fatalf("Should have returned 201, but got %d", wPut0.Code)
	}
	////// GET secret
	if sqlU0, err := h.mag.UserManagement.GetByUsername(ctx, "test"); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, 1, len(sqlU0.Secrets))
	}
	// Get Session
	/* TODO: Can not add cookie to request?
	wGetSession, rGetSession := utils.CreateNewHTTPTest(http.MethodGet,
		map[string]string{"id": userID}, map[string]string{},
		map[string]string{
			"loginName": "test",
		},
		"/session",
		nil)
	h.UserGetSession(&wGetSession, rGetSession)
	if wGetSession.Code != http.StatusOK {
		t.Fatalf("Should have returned 200, but got %d", wGetSession.Code)
	}
	*/
	////// TODO: DELETE secret

	////// TODO: Delete User
}
