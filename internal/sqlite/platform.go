package sqlite

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
)

type Platform struct {
	ID       int64  `json:"id"`
	Arch     string `json:"architecture"`
	OS       string `json:"os"`
	Variant  string `json:"variant"`
	Features string `json:"features"`
}

func (p Platform) createTable() (err error) {
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
        "arch" TEXT,
        "os" TEXT,
        "os_version" TEXT,
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
	p.bootstrapData()
	return
}

func (p Platform) bootstrapData() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	_ = ctx
	defer cancel()
	type input struct {
		os       string
		arch     string
		variant  string
		features string
	}

	inputs := []input{
		{"linux", "amd64", "", ""},
		{"linux", "amd64", "", "cpu:cascadelake"},
		{"linux", "amd64", "", "cpu:skylake"},
		{"linux", "amd64", "", "cpu:skylake_avx512"},
		{"linux", "amd64", "", "cpu:icelake"},
		{"linux", "amd64", "", "cpu:zen"},
		{"linux", "amd64", "", "cpu:zen2"},
		{"linux", "amd64", "", "cpu:zen3"},
		{"linux", "amd64", "", "cpu:broadwell"},
		{"linux", "arm64", "", ""},
		{"linux", "arm", "v6", ""},
		{"linux", "arm", "v7", ""},
		{"linux", "arm64", "", "cpu:graviton2"},
		{"linux", "arm64", "", "cpu:graviton3"},
		{"linux", "riscv64", "", ""},
	}
	cnt := 0
	for _, i := range inputs {
		q := fmt.Sprintf(`INSERT INTO platform (os, arch,variant, features) VALUES ("%s", "%s", "%s", "%s");`, i.os, i.arch, i.variant, i.features)
		res, err := db.Exec(q)
		if err != nil {
			logrus.Error(err)
			continue
		}
		rCnt, _ := res.RowsAffected()
		cnt += int(rCnt)
	}
	logrus.Infof("  Inserted %d rows into table 'platform'", cnt)

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
	return p.bootstrapPlatformFilterData()
}

func (p Platform) bootstrapPlatformFilterData() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	_ = ctx
	defer cancel()
	type plat struct {
		id       int64
		os       string
		arch     string
		variant  string
		features string
	}
	type input struct {
		filter  string
		plaform []plat
	}

	inputs := []input{{
		"generic", []plat{
			{0, "linux", "amd64", "", ""},
			{0, "linux", "arm64", "", ""},
			{0, "linux", "arm", "v6", ""},
			{0, "linux", "arm", "v7", ""},
			{0, "linux", "riscv64", "", ""},
		},
	}, {
		"cascadelake", []plat{
			{0, "linux", "amd64", "", "cpu:cascadelake"},
		},
	}, {
		"skylake", []plat{
			{0, "linux", "amd64", "", "cpu:skylake"},
		},
	}, {
		"skylake_avx512", []plat{
			{0, "linux", "amd64", "", "cpu:skylake_avx512"},
		},
	}, {
		"icelake", []plat{
			{0, "linux", "amd64", "", "cpu:icelake"},
		},
	}, {
		"zen", []plat{
			{0, "linux", "amd64", "", "cpu:zen"},
		},
	}, {
		"zen2", []plat{
			{0, "linux", "amd64", "", "cpu:zen2"},
		},
	}, {
		"zen3", []plat{
			{0, "linux", "amd64", "", "cpu:zen3"},
		},
	}, {
		"broadwell", []plat{
			{0, "linux", "amd64", "", "cpu:broadwell"},
		},
	}, {
		"graviton2", []plat{
			{0, "linux", "arm64", "", "cpu:graviton2"},
		},
	}, {
		"graviton3", []plat{
			{0, "linux", "arm64", "", "cpu:graviton3"},
		},
	},
	}
	cnt := 0
	for _, i := range inputs {
		q := fmt.Sprintf(`SELECT id FROM filter WHERE label = "%s"`, i.filter)
		rows, err := db.Query(q)
		if err != nil {
			logrus.Errorf("Error '%s' while '%s'", err.Error(), q)
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
			for _, p := range i.plaform {
				q := fmt.Sprintf(`SELECT id FROM platform WHERE os = "%s" AND arch = "%s"`, p.os, p.arch)
				if p.variant != "" {
					q += fmt.Sprintf(` AND variant = "%s"`, p.variant)
				}
				if p.features != "" {
					q += fmt.Sprintf(` AND features = "%s"`, p.features)
				}
				q += ";"
				//fmt.Printf("  %s\n", q)
				rows, err := db.Query(q)
				if err != nil {
					logrus.Error(err)
					continue
				}
				if err != nil {
					logrus.Error(err)
					continue
				}
				var platID int
				if rows.Next() {
					err = rows.Scan(&platID)
					if err != nil {
						logrus.Error(err)
						continue
					}
					rows.Close()
					q = fmt.Sprintf(`INSERT INTO platform_filter (platform_id, filter_id) VALUES (%d, %d);`, platID, filterID)
					//fmt.Printf("  %s\n", q)
					res, err := db.Exec(q)
					if err != nil {
						logrus.Error(err)
						continue
					}
					rCnt, _ := res.RowsAffected()
					cnt += int(rCnt)
				}
			}
		}
	}
	logrus.Infof("  Inserted %d rows into table 'platform'", cnt)

	return
}
