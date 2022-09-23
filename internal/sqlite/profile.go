package sqlite

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

type Profile struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
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
	p.bootstrapData()
	return
}

func (p Profile) bootstrapData() (err error) {
	stmt := `INSERT INTO 
			profile(name) 
		VALUES
		('broadwell'),
		('cascadelake'),
		('generic'),
		('graviton2'),
		('icelake'),
		('skylake'),
		('skylake_avx512'),
		('zen'),
		('zen2'),
		('zen3');
	`
	query, err := db.Prepare(stmt)
	if err != nil {
		logrus.Error(err)
	}
	res, err := query.Exec()
	if err != nil {
		logrus.Error(err)
	}
	rCnt, _ := res.RowsAffected()
	logrus.Infof("Inserted %d rows into table 'profile'", rCnt)
	return
}
