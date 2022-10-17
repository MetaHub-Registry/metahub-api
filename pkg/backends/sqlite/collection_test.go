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
	assert.Len(t, cols, 7)
}

func Test_GetCollectionsByRepo(t *testing.T) {
	cols, err := th.Collection.GetCollectionByRepo("")
	assert.NoError(t, err)
	assert.Len(t, cols, 7)
}

func Test_GetCollectionByID(t *testing.T) {
	col, err := th.Collection.GetCollectionByID(1)
	assert.NoError(t, err)
	assert.Equal(t, "biocontainers/blast", col.Repo)
	assert.Equal(t, "2.13.0--hf3cf87c_0", col.Tag)
}
