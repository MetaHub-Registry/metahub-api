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
	TagID int64 `json:"tag_id"`
}

func (rpl ReqPayload) ToByte() []byte {
	newFsConfigBytes, _ := json.Marshal(rpl)
	return newFsConfigBytes
}

func Test_CollectionGetOne(t *testing.T) {
	testRepo := "qnib"
	testImage := "featuretest"
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("filter")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("platforms:arm64")
	bs.AddBootstrap("profiles")
	bs.AddBootstrap("config:ft_arm")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("layer0:ft_arm64")
	bs.AddBootstrap("layer1:ft_arm64")
	bs.AddBootstrap("layer0:ft_amd64")
	bs.AddBootstrap("layer1:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("collection:ft_generic")
	bs.AddBootstrap("collection:ft_arm64")
	//bs.PostTestSleep = 180
	br, err := SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	_ = br
	/// Request
	token, err := br.Get("user:test#bearer")
	assert.NoError(t, err)
	colID, err := br.GetInt64("collection:ft#id")
	assert.NoError(t, err)
	colTagID, err := br.GetInt64("tag:arm64#id")
	assert.NoError(t, err)
	// Tests
	reqBody := ReqPayload{
		ColID: colID,
		TagID: colTagID,
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
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("filter")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("platforms:arm64")
	bs.AddBootstrap("profiles")
	bs.AddBootstrap("config:ft_arm")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("layer0:ft_arm64")
	bs.AddBootstrap("layer1:ft_arm64")
	bs.AddBootstrap("layer0:ft_amd64")
	bs.AddBootstrap("layer1:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("collection:ft_amd64")
	bs.AddBootstrap("collection:ft_arm64")
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
