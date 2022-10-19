package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-api/pkg/types"
)

func Test_GetTypesManifestListByRequest(t *testing.T) {
	req := types.Request{
		Repo:    `gromacs/mpich`,
		Tag:     `2021.5`,
		Profile: "generic",
	}
	ml, found, err := th.ManifestList.GetTypesManifestListByRequest(req)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "sha256:56c58e4bfaa46bae541e90dcd7f24f35dc5f8116e109995af0a05b67df6dff17", ml.Digest)
	expID := ml.SqlID
	req.Tag = ""
	req.Digest = ml.Digest
	ml, found, err = th.ManifestList.GetTypesManifestListByRequest(req)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, expID, ml.SqlID)

}
