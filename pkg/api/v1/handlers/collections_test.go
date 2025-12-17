package handlers

import (
	"context"
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

func TestCollectionGetOne(t *testing.T) {
	testRepo := "qnib"
	testImage := "featuretest"
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	//bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	//bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("collection:ft_generic")
	bs.AddBootstrap("collection:ft_arm64")
	//bs.PostTestSleep = 180
	ctx := context.Background()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs)
	_ = br
	/// Request
	token, err := br.Get("user:test#bearer")
	if err != nil {
		t.Fatal(err)
	}
	colID, err := br.GetInt64("collection:ft#id")
	if err != nil {
		t.Fatal(err)
	}
	colTagID, err := br.GetInt64("tag:arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	// Tests
	testProfile := "generic"
	reqBody := ReqPayload{
		ColID: colID,
		TagID: colTagID,
	}
	wGet := httptest.NewRecorder()
	_, rGet := utils.CreateNewHTTPTest(http.MethodGet, map[string]string{
		"repo":  testRepo,
		"image": testImage,
	}, map[string]string{
		"accountName": token,
		"userName":    testRepo,
		"profileName": testProfile,
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/octet-stream",
	}, "/{repo}/{image}/blobs/uploads/{uuid}", reqBody.ToByte())
	h.CollectionGetOne(wGet, rGet)
	assert.Equal(t, http.StatusOK, wGet.Code)
	var resp CollectionResponse
	err = json.Unmarshal(wGet.Body.Bytes(), &resp)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "qnib/featuretest", resp.Collection.Repo)
	assert.Equal(t, "arm64", resp.Collection.Tag)
}

// func TestCollectionGetAll(t *testing.T) {
// 	bs := utils.NewBootstrap()
// 	bs.SetEmptyAll()
// 	bs.SetYAMLPrefix("../../../../misc/yaml")
// 	bs.AddBootstrap("users")
// 	bs.AddBootstrap("platforms:generic")
// 	bs.AddBootstrap("rules:rules/generic/linux.yaml")
// 	bs.AddBootstrap("profile:profiles/generic.yaml")
// 	bs.AddBootstrap("config:ft_arm64")
// 	bs.AddBootstrap("config:ft_amd64")
// 	bs.AddBootstrap("upstream:ft_arm64")
// 	bs.AddBootstrap("upstream:ft_amd64")
// 	bs.AddBootstrap("manifest:ft_arm64")
// 	bs.AddBootstrap("manifest:ft_amd64")
// 	bs.AddBootstrap("layer:0:ft_arm64")
// 	bs.AddBootstrap("layer:1:ft_arm64")
// 	bs.AddBootstrap("layer:0:ft_amd64")
// 	bs.AddBootstrap("layer:1:ft_amd64")
// 	bs.AddBootstrap("manifestdata:ft_arm64")
// 	bs.AddBootstrap("manifestdata:ft_amd64")
// 	bs.AddBootstrap("collection:ft_amd64")
// 	bs.AddBootstrap("collection:ft_arm64")
// 	bs.AddBootstrap("collection:ft_generic")
// 	bs.AddBootstrap("computeManifests")
// 	bs.AddBootstrap("computeManifestlists")
// 	//bs.PostTestSleep = 300
// 	ctx := context.Background()
// 	br, err := SetupTest(ctx, bs)
// 	if err != nil {
// 		t.Log(err)
// 		TeardownTest(bs)
// 		t.Fatal(err)
// 	}
// 	defer TeardownTest(bs)
// 	_ = br
// 	testRepo := "qnib"
// 	testProfile := "generic"
// 	/// Request
// 	token, err := br.Get("user:test#bearer")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	// Tests
// 	reqBody := ReqRepo{
// 		Repo: "qnib",
// 	}
// 	wGet := httptest.NewRecorder()
// 	_, rGet := utils.CreateNewHTTPTest(http.MethodGet, map[string]string{}, map[string]string{
// 		"accountName": token,
// 		"userName":    testRepo,
// 		"profileName": testProfile,
// 	}, map[string]string{
// 		"Authorization": fmt.Sprintf("Bearer %s", token),
// 		"Content-Type":  "application/octet-stream",
// 	}, "/{repo}/{image}/blobs/uploads/{uuid}", reqBody.ToByte())
// 	h.CollectionGetAll(wGet, rGet)
// 	if wGet.Code != http.StatusOK {
// 		t.Fatalf("Expected %d, got %d", http.StatusOK, wGet.Code)
// 	}
// 	var resp CollectionsResponse
// 	err = json.Unmarshal(wGet.Body.Bytes(), &resp)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	assert.Equal(t, 0, resp.SlugEnd)
// 	assert.Len(t, resp.Next, 1)
// 	// qnib/featuretest
// 	reqBodyFT := ReqRepo{
// 		Repo: "qnib/featuretest",
// 	}
// 	wGetFT := httptest.NewRecorder()
// 	_, rGetFT := utils.CreateNewHTTPTest(http.MethodGet, map[string]string{}, map[string]string{
// 		"accountName": token,
// 		"userName":    testRepo,
// 		"profileName": testProfile,
// 	}, map[string]string{
// 		"Authorization": fmt.Sprintf("Bearer %s", token),
// 		"Content-Type":  "application/octet-stream",
// 	}, "/api/v1/collections", reqBodyFT.ToByte())
// 	h.CollectionGetAll(wGetFT, rGetFT)
// 	if wGetFT.Code != http.StatusOK {
// 		t.Fatalf("Expected %d, got %d", http.StatusOK, wGetFT.Code)
// 	}
// 	var respFT CollectionsResponse
// 	body := wGetFT.Body.Bytes()
// 	err = json.Unmarshal(body, &respFT)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if false {
// 		var prettyJSON bytes.Buffer
// 		error := json.Indent(&prettyJSON, body, "", "\t")
// 		if error != nil {
// 			t.Fatalf("error indenting json: %v", error)
// 		}
// 		fmt.Println(prettyJSON.String())
// 	}
// 	if len(respFT.Next) != 3 {
// 		t.Fatalf("Expected 3, got %d", len(respFT.Next))
// 	}
// 	_, ok := respFT.Next["generic"]
// 	if !ok {
// 		t.Fatalf("Expected 'latest' key in Next: %v", respFT.Next)
// 	}
// 	n := respFT.Next["generic"]
// 	assert.Equal(t, "qnib/featuretest", n.Repo)
// 	assert.Equal(t, "generic", n.Tag)
// }
