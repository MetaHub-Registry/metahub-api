package sqlite

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
)

type Manifest struct {
	ID           int64    `json:"id"`
	Digest       string   `json:"digest"`
	ImageName    string   `json:"image_name"`
	PlatformSpec Platform `json:"platform_spec"`
	Upstream     Upstream `json:"upstream"`
	Config       Config   `json:"config"`
	Profile      Profile  `json:"profile"`
}

func (m Manifest) createTable() (err error) {
	tableName := "manifest"
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
		"image_name" TEXT,
		"platform_id" int,
        "upstream_id" TEXT,
        "config_id" INT,
		"profile_id" INT,
		FOREIGN KEY (platform_id)
			REFERENCES platform (id)
		FOREIGN KEY (upstream_id)
			REFERENCES upstream (id) 
		FOREIGN KEY (config_id)
			REFERENCES config (id)
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
	m.bootstrapData()
	return
}

func (m Manifest) bootstrapData() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	_ = ctx
	defer cancel()
	type plat struct {
		os       string
		arch     string
		variant  string
		features string
	}
	type input struct {
		name string
		reg  string
		repo string
		tag  string
		plat plat
	}

	inputs := []input{
		{
			"quay.io/cqnib/gromacs/gcc-7.3.1/2021.5/tmpi:skylake_avx512", "quay.io", "cqnib/gromacs/gcc-7.3.1/2021.5/tmpi", "skylake_avx512",
			plat{"linux", "amd64", "", "cpu:skylake_avx512"},
		}, {
			"quay.io/cqnib/gromacs/gcc-7.3.1/2021.5/tmpi:graviton2", "quay.io", "cqnib/gromacs/gcc-7.3.1/2021.5/tmpi", "graviton2",
			plat{"linux", "arm64", "", "cpu:graviton2"},
		}, {
			"quay.io/cqnib/plain-featuretest:generic", "quay.io", "cqnib/plain-featuretest", "generic",
			plat{"linux", "amd64", "", ""},
		}, {
			"quay.io/cqnib/plain-featuretest:arch-cascadelake", "quay.io", "cqnib/plain-featuretest", "arch-cascadelake",
			plat{"linux", "amd64", "", "cpu:cascadelake"},
		}, {
			"quay.io/cqnib/plain-featuretest:arch-skylake_avx512", "quay.io", "cqnib/plain-featuretest", "arch-skylake_avx512",
			plat{"linux", "amd64", "", "cpu:skylake_avx512"},
		}, {
			"quay.io/cqnib/plain-featuretest:arch-skylake", "quay.io", "cqnib/plain-featuretest", "arch-skylake",
			plat{"linux", "amd64", "", "cpu:skylake"},
		}, {
			"quay.io/cqnib/plain-featuretest:arch-zen", "quay.io", "cqnib/plain-featuretest", "arch-zen",
			plat{"linux", "amd64", "", "cpu:zen"},
		}, {
			"quay.io/cqnib/plain-featuretest:arch-zen2", "quay.io", "cqnib/plain-featuretest", "arch-zen2",
			plat{"linux", "amd64", "", "cpu:zen2"},
		}, {
			"quay.io/cqnib/plain-featuretest:arch-zen3", "quay.io", "cqnib/plain-featuretest", "arch-zen3",
			plat{"linux", "amd64", "", "cpu:skylake_avx512"},
		}, {
			"quay.io/cqnib/plain-featuretest:cpu-broadwell", "quay.io", "cqnib/plain-featuretest", "cpu-broadwell",
			plat{"linux", "amd64", "", "cpu:broadwell"},
		}, {
			"registry.gitlab.com/qnib-pub-containers/qnib/plain-featuretest:20220715-arm64-generic", "registry.gitlab.com", "qnib-pub-containers/qnib/plain-featuretest", "20220715-arm64-generic",
			plat{"linux", "arm64", "", ""},
		}, {
			"registry.gitlab.com/qnib-pub-containers/qnib/plain-featuretest:20220715-arm64-graviton2", "registry.gitlab.com", "qnib-pub-containers/qnib/plain-featuretest", "20220715-arm64-graviton2",
			plat{"linux", "arm64", "", "cpu:graviton2"},
		}, {
			"registry.gitlab.com/qnib-pub-containers/qnib/plain-featuretest:20220715-arm_v6-generic", "registry.gitlab.com", "qnib-pub-containers/qnib/plain-featuretest", "20220715-arm_v6-generic",
			plat{"linux", "arm", "v6", ""},
		}, {
			"registry.gitlab.com/qnib-pub-containers/qnib/plain-featuretest:20220715-arm_v7-generic", "registry.gitlab.com", "qnib-pub-containers/qnib/plain-featuretest", "20220715-arm_v7-generic",
			plat{"linux", "arm", "v7", ""},
		}, {
			"registry.gitlab.com/qnib-pub-containers/qnib/plain-featuretest:20220715-riscv64-generic", "registry.gitlab.com", "qnib-pub-containers/qnib/plain-featuretest", "20220715-riscv64-generic",
			plat{"linux", "riscv64", "", ""},
		}, {
			"quay.io/biocontainers/blast:2.12.0--hf3cf87c_4", "quay.io", "biocontainers/blast", "2.12.0--hf3cf87c_4",
			plat{"linux", "amd64", "", ""},
		}, {
			"quay.io/biocontainers/blast:2.13.0--hf3cf87c_0", "quay.io", "biocontainers/blast", "2.13.0--hf3cf87c_0",
			plat{"linux", "amd64", "", ""},
		}, {
			"registry.gitlab.com/qnib-pub-containers/biocontainers/blast:2-13-0-arm64", "registry.gitlab.com", "qnib-pub-containers/biocontainers/blast", "2-13-0-arm64",
			plat{"linux", "arm64", "", ""},
		}, {
			"quay.io/cqnib/gromacs-2021.5_gcc-7.3.1:x86_64_v4", "quay.io", "cqnib/gromacs-2021.5_gcc-7.3.1", "x86_64_v4",
			plat{"linux", "amd64", "", ""},
		}, {
			"quay.io/cqnib/gromacs-2021.5_gcc-11.3.0:zen3", "quay.io", "cqnib/gromacs-2021.5_gcc-11.3.0", "zen3",
			plat{"linux", "amd64", "", "cpu:zen3"},
		}, {
			"quay.io/cqnib/gromacs-2021.5_gcc-7.3.1:aarch64", "quay.io", "cqnib/gromacs-2021.5_gcc-7.3.1", "aarch64",
			plat{"linux", "arm64", "", ""},
		},
	}
	cnt := 0
	for _, i := range inputs {
		q := fmt.Sprintf(`SELECT id FROM platform WHERE os = "%s" AND arch = "%s"`, i.plat.os, i.plat.arch)
		if i.plat.variant != "" {
			q += fmt.Sprintf(` AND variant = "%s"`, i.plat.variant)
		}
		if i.plat.features != "" {
			q += fmt.Sprintf(` AND features = "%s"`, i.plat.features)
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
		} else {
			logrus.Errorf("No platform found for: %v", i.plat)
			continue
		}
		rows.Close()
		q = fmt.Sprintf(`SELECT id FROM upstream WHERE reg = "%s" AND repo = "%s" AND tag = "%s";`, i.reg, i.repo, i.tag)
		//fmt.Printf("  %s\n", q)
		rows, err = db.Query(q)
		if err != nil {
			logrus.Error(err)
			continue
		}
		if err != nil {
			logrus.Error(err)
			continue
		}
		var id int
		if rows.Next() {
			err = rows.Scan(&id)
			if err != nil {
				logrus.Error(err)
				continue
			}
			rows.Close()
			q = fmt.Sprintf(`INSERT INTO manifest (image_name, upstream_id, platform_id) VALUES ("%s", %d, %d);`, i.name, id, platID)
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
	logrus.Infof("Inserted %d rows into table 'manifests'", cnt)

	return
}

func NewManifestFromKeys(keys map[string]interface{}) (mani Manifest, err error) {
	return
}

func (m *Manifest) GetByCollectionId(colId int64) (manis []Manifest, err error) {

	return
}
