package sqlite

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/types"
)

type Upstream struct {
	ID   int64  `json:"id"`
	Reg  string `json:"reg"`
	Repo string `json:"repo"`
	Tag  string `json:"tag"`
}

func (u Upstream) CreateTables() (err error) {
	tableName := "upstream"
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
		"digest" TEXT,
		"last_visited" TIMESTAMP,
        "reg" TEXT,
		"repo" TEXT,
		"tag" TEXT
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

func (u Upstream) Insert(i *types.Upstream) (err error) {
	q := `INSERT INTO upstream (reg, repo, tag) VALUES (?, ?, ?)`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Fatal(err)
	}
	res, err := stmt.Exec(i.Reg, i.Repo, i.Tag)
	if err != nil {
		logrus.Fatal(err)
	}
	i.SqlID, err = res.LastInsertId()
	return
}
