package sqlite

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/metahub-registry/metahub-api/internal/driver"
)

var th TestHelper

type TestHelper struct {
	drvDB      *driver.DB
	Collection Collection
	Config     Config
	Filter     Filter
	Manifest   Manifest
	Platform   Platform
	Profile    Profile
	Upstream   Upstream
}

func NewTestHelper(fname string) TestHelper {
	drvDB, err := driver.ConnectSqlite(fname)
	if err != nil {
		log.Fatal("Cannot connect to database")
	}
	db = drvDB.SQL
	return TestHelper{
		drvDB:      drvDB,
		Collection: Collection{},
		Config:     Config{},
		Filter:     Filter{},
		Manifest:   Manifest{},
		Platform:   Platform{},
		Profile:    Profile{},
		Upstream:   Upstream{},
	}
}

func (th *TestHelper) GetSQL() *sql.DB {
	return th.drvDB.SQL
}

func TestMain(m *testing.M) {
	fmt.Println("TestMain")
	th = NewTestHelper("../../../misc/sqlite/metahub_20220930.db")
	os.Exit(m.Run())
}
