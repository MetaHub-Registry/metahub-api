package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_CreateView(t *testing.T) {
	err := th.Collection.CreateViews()
	assert.NoError(t, err)
}

func Test_GetAllCollections(t *testing.T) {
	cols, err := th.Collection.GetAll()
	assert.NoError(t, err)
	assert.Len(t, cols, 36)
}

func Test_GetCollections(t *testing.T) {
	cols, err := th.Collection.GetCollectionsByRepoLike("gromacs/tmpi")
	assert.NoError(t, err)
	assert.Len(t, cols, 1)
	exp := cols[0]
	col, err := th.Collection.GetCollectionByID(exp.ID)
	assert.NoError(t, err)
	assert.Equal(t, "gromacs/tmpi", col.Repo)
	assert.Equal(t, "2021.5", col.Tag)
	manis, err := th.Collection.GetTypesManifestByCollectionID(exp.ID)
	assert.NoError(t, err)
	assert.Len(t, manis, 4)
	// Tags

	tag, err := th.Collection.GetTagsListByRepo("biocontainers/blast")
	assert.NoError(t, err)
	assert.Len(t, tag.Tags, 4)
}
