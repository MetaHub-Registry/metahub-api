package sqlite

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
)

type Filter struct {
	ID    int64  `json:"id"`
	Label string `json:"label"`
}

func (f Filter) createTable() (err error) {
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
	return f.bootstrapData()
}

func (f Filter) bootstrapData() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	_ = ctx
	defer cancel()
	type input struct {
		os       string
		arch     string
		variant  string
		features string
	}

	inputs := []string{
		"generic",
		"cascadelake",
		"skylake",
		"skylake_avx512",
		"icelake",
		"zen",
		"zen2",
		"zen3",
		"broadwell",
		"graviton2",
		"graviton3",
	}
	cnt := 0
	for _, i := range inputs {
		q := fmt.Sprintf(`INSERT INTO filter (label) VALUES ("%s");`, i)
		res, err := db.Exec(q)
		if err != nil {
			logrus.Error(err)
			continue
		}
		rCnt, _ := res.RowsAffected()
		cnt += int(rCnt)
	}
	logrus.Infof("  Inserted %d rows into table 'filter'", cnt)

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
	return f.bootstrapProfileFilterData()
}

func (f Filter) bootstrapProfileFilterData() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	_ = ctx
	defer cancel()
	type input struct {
		filter  string
		profile string
	}

	inputs := []input{{
		"generic", "generic",
	}, {
		"cascadelake", "cascadelake",
	}, {
		"skylake", "skylake",
	}, {
		"skylake_avx512", "skylake_avx512",
	}, {
		"icelake", "icelake",
	}, {
		"zen", "zen",
	}, {
		"zen2", "zen2",
	}, {
		"zen3", "zen3",
	}, {
		"broadwell", "broadwell",
	}, {
		"graviton2", "graviton2",
	}, {
		"graviton3", "graviton3",
	},
	}
	cnt := 0
	for _, i := range inputs {
		q := fmt.Sprintf(`SELECT id FROM filter WHERE label = "%s"`, i.filter)
		rows, err := db.Query(q)
		if err != nil {
			logrus.Error(err)
			continue
		}
		if err != nil {
			logrus.Error(err)
			continue
		}
		var filterID int
		if rows.Next() {
			err = rows.Scan(&filterID)
			if err != nil {
				logrus.Error(err)
				continue
			}
			rows.Close()
		}
		q = fmt.Sprintf(`SELECT id FROM profile WHERE name = "%s"`, i.profile)
		rows, err = db.Query(q)
		if err != nil {
			logrus.Error(err)
			continue
		}
		if err != nil {
			logrus.Error(err)
			continue
		}
		var profileID int
		if rows.Next() {
			err = rows.Scan(&profileID)
			if err != nil {
				logrus.Error(err)
				continue
			}
			rows.Close()
		}
		q = fmt.Sprintf(`INSERT INTO profile_filter (profile_id, filter_id) VALUES (%d, %d);`, profileID, filterID)
		//fmt.Printf("  %s\n", q)
		res, err := db.Exec(q)
		if err != nil {
			logrus.Error(err)
			continue
		}
		rCnt, _ := res.RowsAffected()
		cnt += int(rCnt)
	}
	return
}
