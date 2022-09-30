package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ModelsCreateViews(t *testing.T) {
	th := NewTestHelper("../../misc/sqlite/metahub_20220930.db")
	m, err := New(th.GetSQL())
	assert.NoError(t, err)
	_ = m
}
