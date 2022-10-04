package sqlite

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/types"
)

type Platform struct {
	ID       int64  `json:"id"`
	Arch     string `json:"architecture"`
	OS       string `json:"os"`
	Variant  string `json:"variant"`
	Features string `json:"features"`
}

func (p Platform) CreateTables() (err error) {
	err = p.createPlatformTable()
	if err != nil {
		logrus.Error(err)
		return
	}
	err = p.createPlatformFilterXTable()
	if err != nil {
		logrus.Error(err)
		return
	}
	return
}

func (p Platform) createPlatformTable() (err error) {
	tableName := "platform"
	q := fmt.Sprintf(`PRAGMA table_info(%s)`, tableName)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		logrus.Infof("Table '%s' already exists!", tableName)
		return
	}
	s := `CREATE TABLE IF NOT EXISTS %s (
        id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
        "os" TEXT,
        "os_version" TEXT,
        "arch" TEXT,
        "variant" TEXT,
        "features" TEXT
		);`
	stmt := fmt.Sprintf(s, tableName)
	query, err := db.Prepare(stmt)
	if err != nil {
		logrus.Fatal(err)
	}
	_, err = query.Exec()
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Infof("Table '%s' created successfully!", tableName)
	return
}

func (p Platform) createPlatformFilterXTable() (err error) {
	tableName := "platform_filter"
	q := fmt.Sprintf(`PRAGMA table_info(%s)`, tableName)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		logrus.Infof("Table '%s' already exists!", tableName)
		return
	}
	s := `CREATE TABLE IF NOT EXISTS %s (
        id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
        "filter_id" INT,
		"platform_id" INT,
		FOREIGN KEY (filter_id)
			REFERENCES filter (id)
		FOREIGN KEY (platform_id)
			REFERENCES platform (id)
		);`
	stmt := fmt.Sprintf(s, tableName)
	query, err := db.Prepare(stmt)
	if err != nil {
		logrus.Fatal(err)
	}
	_, err = query.Exec()
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Infof("Table '%s' created successfully!", tableName)
	return
}

func (p Platform) Insert(i *types.Platform) (err error) {
	q := `INSERT INTO platform (os, arch, variant, features) VALUES (?, ?, ?, ?)`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Fatal(err)
	}
	res, err := stmt.Exec(i.OS, i.Arch, i.Variant, i.Features)
	if err != nil {
		logrus.Fatal(err)
	}
	i.SqlID, err = res.LastInsertId()
	return
}

func (p Platform) Upsert(i *types.Platform) (err error) {
	// Already exists?
	q := `SELECT id FROM platform WHERE os=? AND arch=? AND variant=? AND features=?`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Fatal(err)
	}
	var id int64
	rows, err := stmt.Query(i.OS, i.Arch, i.Variant, i.Features)
	if err != nil {
		logrus.Error(err)
		return
	}
	defer rows.Close()
	if rows.Next() {
		err = rows.Scan(&id)
		if err != nil {
			logrus.Error(err)
			return
		}
		i.SqlID = id
		return
	}
	// Not present, insert new Platform
	return p.Insert(i)
}

func (p Platform) InsertPlatformFilter(pfp types.ProfileFilterPlatform) (err error) {
	if pfp.Platform.SqlID == 0 {
		logrus.Fatal("Platform.SqlID is 0")
	}
	if pfp.Filter.SqlID == 0 {
		logrus.Fatal("Filter.SqlID is 0")
	}

	stmt, err := db.Prepare("INSERT INTO platform_filter (platform_id, filter_id) VALUES (?, ?)")
	if err != nil {
		logrus.Fatal(err)
	}
	_, err = stmt.Exec(pfp.Platform.SqlID, pfp.Filter.SqlID)
	if err != nil {
		logrus.Fatal(err)
	}
	return
}
