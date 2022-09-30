package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ManifestCreateView(t *testing.T) {
	err := th.Manifest.CreateViews()
	assert.NoError(t, err)
}

func Test_GetByCollectionId(t *testing.T) {
	manis, err := th.Manifest.GetByCollectionId(1)
	assert.NoError(t, err)
	assert.Len(t, manis, 2)
}
