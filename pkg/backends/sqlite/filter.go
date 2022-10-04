package sqlite

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/types"
)

type Filter struct {
	ID    int64  `json:"id"`
	Label string `json:"label"`
}

func (f Filter) CreateTables() (err error) {
	err = f.createFilterTable()
	if err != nil {
		logrus.Error(err)
		return
	}
	err = f.createProfileFilterXTable()
	if err != nil {
		logrus.Error(err)
		return
	}
	return
}

func (f Filter) createFilterTable() (err error) {
	tableName := "filter"
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
        "label" TEXT
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

func (f Filter) createProfileFilterXTable() (err error) {
	tableName := "profile_filter"
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
        "profile_id" INT,
		"filter_id" INT,
		FOREIGN KEY (filter_id)
			REFERENCES filter (id)
		FOREIGN KEY (profile_id)
			REFERENCES profile (id)
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

func (f Filter) InsertFilter(fltr *types.Filter) (err error) {
	// TODO: Do not insert if already exists?
	stmt, err := db.Prepare("INSERT INTO filter (label) VALUES (?) RETURNING id")
	if err != nil {
		logrus.Fatal(err)
	}
	res, err := stmt.Exec(fltr.Label)
	if err != nil {
		logrus.Fatal(err)
	}
	fltr.SqlID, err = res.LastInsertId()
	return

}

func (f Filter) InsertProfileFilter(pfp types.ProfileFilterPlatform) (err error) {
	stmt, err := db.Prepare("INSERT INTO profile_filter (profile_id, filter_id) VALUES (?, ?)")
	if err != nil {
		logrus.Fatal(err)
	}
	_, err = stmt.Exec(pfp.Profile.SqlID, pfp.Filter.SqlID)
	if err != nil {
		logrus.Fatal(err)
	}
	return
}
