package sqlite

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

type Upstream struct {
	ID   int64  `json:"id"`
	Reg  string `json:"reg"`
	Repo string `json:"repo"`
	Tag  string `json:"tag"`
}

func (u Upstream) createTable() (err error) {
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
	u.bootstrapData()
	return
}

func (u Upstream) bootstrapData() (err error) {
	stmt := `INSERT INTO 
			upstream(reg,repo,tag) 
		VALUES
			("quay.io","cqnib/gromacs/gcc-7.3.1/2021.5/tmpi","skylake_avx512"),
			("quay.io","cqnib/gromacs/gcc-7.3.1/2021.5/tmpi","graviton2"),
			("quay.io","cqnib/plain-featuretest","generic"),
			("quay.io","cqnib/plain-featuretest","arch-cascadelake"),
			("quay.io","cqnib/plain-featuretest","arch-skylake"),
			("quay.io","cqnib/plain-featuretest","arch-skylake_avx512"),
			("quay.io","cqnib/plain-featuretest","arch-zen"),
			("quay.io","cqnib/plain-featuretest","arch-zen2"),
			("quay.io","cqnib/plain-featuretest","arch-zen3"),
			("quay.io","cqnib/plain-featuretest","cpu-broadwell"),
			("registry.gitlab.com","qnib-pub-containers/qnib/plain-featuretest","20220715-arm64-generic"),
			("registry.gitlab.com","qnib-pub-containers/qnib/plain-featuretest","20220715-arm64-graviton2"),
			("registry.gitlab.com","qnib-pub-containers/qnib/plain-featuretest","20220715-arm_v6-generic"),
			("registry.gitlab.com","qnib-pub-containers/qnib/plain-featuretest","20220715-arm_v7-generic"),
			("registry.gitlab.com","qnib-pub-containers/qnib/plain-featuretest","20220715-riscv64-generic"),
			("quay.io","biocontainers/blast","2.12.0--hf3cf87c_4"),
			("quay.io","biocontainers/blast","2.13.0--hf3cf87c_0"),
			("registry.gitlab.com","qnib-pub-containers/biocontainers/blast","2-13-0-arm64"),
			("quay.io","cqnib/gromacs-2021.5_gcc-7.3.1","x86_64_v4"),
			("quay.io","cqnib/gromacs-2021.5_gcc-11.3.0","zen3"),
			("quay.io","cqnib/gromacs-2021.5_gcc-7.3.1","aarch64")
		;
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
