package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_ListTags(t *testing.T) {
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
	wList0, rList0 := createNewHTTPTest(http.MethodGet, map[string]string{
		"repo":  testRepo,
		"image": testImage,
	}, map[string]string{
		"accountName": token,
		"userName":    "test",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}, "/{repo}/{image}/tags/list", nil)

	h.TagGetHandler(&wList0, rList0)
	assert.Equal(t, http.StatusOK, wList0.Code)
	//
	var list0 types.TagResponse
	err = json.Unmarshal(wList0.Body.Bytes(), &list0)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(list0.Tags))
}
