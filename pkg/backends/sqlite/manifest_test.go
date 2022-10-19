package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-api/pkg/types"
)

func Test_GetManifest(t *testing.T) {
	req := types.Request{
		Digest: "sha256:3f6ad3aa7db403b67849bcdca47449708707e7fb663b419b478f1acf94115cae",
	}
	m, found, err := th.Manifest.GetTypesManifestByRequest(req)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "cqnib/gromacs-2021.5_gcc-7.3.1", m.Upstream.Repo)
	assert.Equal(t, "x86_64_v4", m.Upstream.Tag)
	cols, err := th.Collection.GetCollectionsByRepoLike("gromacs/tmpi")
	assert.NoError(t, err)
	assert.Len(t, cols, 1)
	exp := cols[0]
	manis, err := th.Manifest.GetByCollectionId(exp.ID)
	assert.NoError(t, err)
	assert.Len(t, manis, 4)
}

func Test_GetManifestList(t *testing.T) {
	manis, err := th.Manifest.TypesManifest_GetByManifestListID(2)
	assert.NoError(t, err)
	assert.Len(t, manis, 1)
}
