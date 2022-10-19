package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Models(t *testing.T) {
	th := NewTestHelper("../../../misc/sqlite/metahub_20221018-1.db")
	m, err := New(th.GetSQL())
	assert.NoError(t, err)
	err = m.LoginUser("admin", "generic", "admin")
	assert.Error(t, err)
	err = m.LoginUser("meta", "generic", "FALSE")
	assert.Error(t, err)
	err = m.LoginUser("meta", "generic", "hub")
	assert.NoError(t, err)

}
