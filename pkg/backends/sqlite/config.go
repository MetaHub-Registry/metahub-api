package sqlite

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/types"
)

type Config struct {
	ID        int64  `json:"id"`
	Digest    string `json:"digest"`
	Label     string `json:"label"`
	MediaType string `json:"media_type"`
	Size      int    `json:"size"`
}

func (c Config) CreateTables() (err error) {
	tableName := "config"
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
        "label" TEXT,
        "media_type" TEXT,
        "size" INT);`
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

func (c Config) Insert(i *types.Config) (err error) {
	q := `INSERT INTO config (digest, label, media_type, size) VALUES (?, ?, ?, ?)`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	res, err := stmt.Exec(i.Digest, i.Label, i.MediaType, i.Size)
	if err != nil {
		logrus.Error(err)
		return
	}
	i.SqlID, err = res.LastInsertId()
	return
}
