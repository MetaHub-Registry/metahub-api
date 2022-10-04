package sqlite

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/types"
)

type Profile struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func (p Profile) CreateTables() (err error) {
	return p.createTable()
}

func (p Profile) createTable() (err error) {
	tableName := "profile"
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
	q = `CREATE TABLE IF NOT EXISTS %s (
        id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
        "name" TEXT NOT NULL UNIQUE
		);`
	stmt := fmt.Sprintf(q, tableName)
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

func (p Profile) HasProfile(name string) (profileID int64, err error) {
	q := `SELECT id FROM profile WHERE name = ?`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	rows, err := stmt.Query(name)
	if err != nil {
		logrus.Error(err)
		return
	}
	defer rows.Close()
	if rows.Next() {
		err = rows.Scan(&profileID)
		if err != nil {
			logrus.Error(err)
		}
	}
	return
}

func (p Profile) Insert(prf *types.Profile) (err error) {
	// TODO: Do not insert if already exists?
	stmt, err := db.Prepare("INSERT INTO profile (name) VALUES (?) RETURNING id")
	if err != nil {
		logrus.Fatal(err)
	}
	res, err := stmt.Exec(prf.Name)
	if err != nil {
		logrus.Fatal(err)
	}
	prf.SqlID, err = res.LastInsertId()
	return
}
