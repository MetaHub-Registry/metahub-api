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

func Test_GetCollectionsByRepo(t *testing.T) {
	cols, err := th.Collection.GetCollectionsByRepoLike("")
	assert.NoError(t, err)
	assert.Len(t, cols, 8)
}

func Test_GetCollectionByID(t *testing.T) {
	col, err := th.Collection.GetCollectionByID(1)
	assert.NoError(t, err)
	assert.Equal(t, "qnib/featuretest", col.Repo)
	assert.Equal(t, "latest", col.Tag)
}
