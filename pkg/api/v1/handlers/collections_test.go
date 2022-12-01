package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	apiutils "github.com/metahub-registry/metahub-api/internal/utils"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

type CollectionResponse struct {
	Repos      []string              `json:"repos"`
	BCrumbs    []apiutils.BreadCrumb `json:"bread_crumbs"`
	Collection types.Collection      `json:"collection"`
}

type CollectionsResponse struct {
	Next    map[string]Next       `json:"next"`
	SlugEnd int                   `json:"slug_end"`
	Repos   []string              `json:"repos"`
	BCrumbs []apiutils.BreadCrumb `json:"bread_crumbs"`
}

type ReqRepo struct {
	Repo string `json:"repo"`
}

func (rr ReqRepo) ToByte() []byte {
	b, _ := json.Marshal(rr)
	return b
}

type ReqPayload struct {
	ColID int64 `json:"collection_id"`
}

func (rpl ReqPayload) ToByte() []byte {
	newFsConfigBytes, _ := json.Marshal(rpl)
	return newFsConfigBytes
}

func Test_CollectionGetOne(t *testing.T) {
	testRepo := "qnib"
	testImage := "featuretest"
	bs := utils.NewBootstrap()
	bs.SetSqlEmptyAll()
	bs.AddBootstrap("users")
	bs.AddBootstrap("profiles")
	bs.AddBootstrap("platforms")
	/// ARM
	bs.AddBootstrap("upstreams:qnib/featuretest:arm64")
	bs.AddBootstrap("layers:qnib/featuretest:arm64")
	bs.AddBootstrap("config:qnib/featuretest:arm64")
	bs.AddBootstrap("manifest:qnib/featuretest:arm64")
	bs.AddBootstrap("manifestlist:qnib/featuretest:arm64")
	bs.AddBootstrap("collection:qnib/featuretest:arm64")
	/// AMD64
	bs.AddBootstrap("upstreams:qnib/featuretest:amd64")
	bs.AddBootstrap("layers:qnib/featuretest:amd64")
	bs.AddBootstrap("config:qnib/featuretest:amd64")
	bs.AddBootstrap("manifest:qnib/featuretest:amd64")
	bs.AddBootstrap("manifestlist:qnib/featuretest:amd64")
	bs.AddBootstrap("collection:qnib/featuretest:amd64")
	//bs.SetSqlite("Desktop/metahub.db")
	br, err := SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	_ = br
	/// Request
	token, err := br.Get("user:test#bearer")
	assert.NoError(t, err)
	// Tests
	reqBody := ReqPayload{
		ColID: 1,
	}
	wGet := httptest.NewRecorder()
	_, rGet := createNewHTTPTest(http.MethodGet, map[string]string{
		"repo":  testRepo,
		"image": testImage,
	}, map[string]string{
		"accountName": token,
		"userName":    "test",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/octet-stream",
	}, "/{repo}/{image}/blobs/uploads/{uuid}", reqBody.ToByte())
	h.CollectionGetOne(wGet, rGet)
	assert.Equal(t, http.StatusOK, wGet.Code)
	var resp CollectionResponse
	err = json.Unmarshal(wGet.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "qnib/featuretest", resp.Collection.Repo)
	assert.Equal(t, "arm64", resp.Collection.Tag)
}
func Test_CollectionGetAll(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetSqlEmptyAll()
	bs.AddBootstrap("users")
	bs.AddBootstrap("profiles")
	bs.AddBootstrap("platforms")
	/// ARM
	bs.AddBootstrap("upstreams:qnib/featuretest:arm64")
	bs.AddBootstrap("layers:qnib/featuretest:arm64")
	bs.AddBootstrap("config:qnib/featuretest:arm64")
	bs.AddBootstrap("manifest:qnib/featuretest:arm64")
	bs.AddBootstrap("manifestlist:qnib/featuretest:arm64")
	bs.AddBootstrap("collection:qnib/featuretest:arm64")
	/// AMD64
	bs.AddBootstrap("upstreams:qnib/featuretest:amd64")
	bs.AddBootstrap("layers:qnib/featuretest:amd64")
	bs.AddBootstrap("config:qnib/featuretest:amd64")
	bs.AddBootstrap("manifest:qnib/featuretest:amd64")
	bs.AddBootstrap("manifestlist:qnib/featuretest:amd64")
	bs.AddBootstrap("collection:qnib/featuretest:amd64")
	//bs.SetSqlite("Desktop/metahub.db")
	br, err := SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	_ = br
	/// Request
	token, err := br.Get("user:test#bearer")
	assert.NoError(t, err)
	// Tests
	reqBody := ReqRepo{
		Repo: "qnib",
	}
	wGet := httptest.NewRecorder()
	_, rGet := createNewHTTPTest(http.MethodGet, map[string]string{}, map[string]string{
		"accountName": token,
		"userName":    "test",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/octet-stream",
	}, "/{repo}/{image}/blobs/uploads/{uuid}", reqBody.ToByte())
	h.CollectionGetAll(wGet, rGet)
	assert.Equal(t, http.StatusOK, wGet.Code)
	var resp CollectionsResponse
	err = json.Unmarshal(wGet.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.SlugEnd)
	assert.Len(t, resp.Next, 1)
}
