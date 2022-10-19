package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Layer_GetByHTTPRequest(t *testing.T) {
	l, found, err := th.Layer.LayerGetByDigest("none")
	assert.NoError(t, err)
	assert.False(t, found)
	assert.Equal(t, "", l.Digest)
	l, found, err = th.Layer.LayerGetByDigest("sha256:525b43075a1835aa8f06fb5b18b985bbf5b38dd530c3c88e1e7dbb93df5b78f0")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "sha256:525b43075a1835aa8f06fb5b18b985bbf5b38dd530c3c88e1e7dbb93df5b78f0", l.Digest)
}
