package sqlite

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
)

type Collection struct {
	ID            int64               `json:"id"`
	Repo          string              `json:"repo"`
	Tag           string              `json:"tag"`
	Manifests     map[string]Manifest `json:"manifests"`
	ManifestCount int64               `json:"manifest_count"`
}

type colRes struct {
	ColID    int    `json:"collection_id"`
	ColRepo  string `json:"collection_repo"`
	ColTag   string `json:"collection_tag"`
	ManiID   int    `json:"manifest_id"`
	UpReg    string `json:"upstream_reg"`
	UpRepo   string `json:"upstream_repo"`
	UpTag    string `json:"upstream_tag"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Variant  string `json:"variant"`
	Features string `json:"features"`
	Profile  string `json:"profile"`
}

func (c Collection) createCollectionTable() (err error) {
	tableName := "collection"
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
        "repo" TEXT,
		"tag" int 
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

func (c Collection) createCollectionManifestTable() (err error) {
	tableName := "col_manifest"
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
        "collection_id" INT,
		"manifest_id" INT,
		FOREIGN KEY (collection_id)
			REFERENCES collection (id)
		FOREIGN KEY (manifest_id)
			REFERENCES manifest (id) 
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
	c.bootstrapCollectionData()
	return
}

func (c Collection) bootstrapCollectionData() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	_ = ctx
	defer cancel()
	type manifest struct {
		imageName string
	}
	type input struct {
		colRepo string
		colTag  string
		mani    []manifest
	}

	inputs := []input{
		{
			"gromacs/tmpi", "2021.5", []manifest{
				{"quay.io/cqnib/gromacs/gcc-7.3.1/2021.5/tmpi:skylake_avx512"},
				{"quay.io/cqnib/gromacs/gcc-7.3.1/2021.5/tmpi:graviton2"},
				{"quay.io/cqnib/gromacs-2021.5_gcc-7.3.1:x86_64_v4"},
				{"quay.io/cqnib/gromacs-2021.5_gcc-7.3.1:aarch64"},
			},
		}, {
			"qnib/featuretest", "latest", []manifest{
				{"quay.io/cqnib/plain-featuretest:generic"},
				{"quay.io/cqnib/plain-featuretest:arch-cascadelake"},
				{"quay.io/cqnib/plain-featuretest:arch-skylake"},
				{"quay.io/cqnib/plain-featuretest:arch-skylake_avx512"},
				{"quay.io/cqnib/plain-featuretest:arch-zen"},
				{"quay.io/cqnib/plain-featuretest:arch-zen2"},
				{"quay.io/cqnib/plain-featuretest:arch-zen3"},
				{"quay.io/cqnib/plain-featuretest:cpu-broadwell"},
				{"registry.gitlab.com/qnib-pub-containers/qnib/plain-featuretest:20220715-arm64-generic"},
				{"registry.gitlab.com/qnib-pub-containers/qnib/plain-featuretest:20220715-arm64-graviton2"},
				{"registry.gitlab.com/qnib-pub-containers/qnib/plain-featuretest:20220715-arm_v6-generic"},
				{"registry.gitlab.com/qnib-pub-containers/qnib/plain-featuretest:20220715-arm_v7-generic"},
				{"registry.gitlab.com/qnib-pub-containers/qnib/plain-featuretest:20220715-riscv64-generic"},
			},
		}, {
			"biocontainers/blast", "2.12.0--hf3cf87c_4", []manifest{
				{"quay.io/biocontainers/blast:2.12.0--hf3cf87c_4"},
			},
		}, {
			"biocontainers/blast", "2.13.0", []manifest{
				{"quay.io/biocontainers/blast:2.13.0--hf3cf87c_0"},
				{"registry.gitlab.com/qnib-pub-containers/biocontainers/blast:2-13-0-arm64"},
			},
		}, {
			"biocontainers/blast", "2.13.0--hf3cf87c_0", []manifest{
				{"quay.io/biocontainers/blast:2.13.0--hf3cf87c_0"},
				{"registry.gitlab.com/qnib-pub-containers/biocontainers/blast:2-13-0-arm64"},
			},
		}, {
			"gromacs/mpich", "2021.5", []manifest{
				{"quay.io/cqnib/gromacs-2021.5_gcc-7.3.1:x86_64_v4"},
				{"quay.io/cqnib/gromacs-2021.5_gcc-11.3.0:zen3"},
				{"quay.io/cqnib/gromacs-2021.5_gcc-7.3.1:aarch64"},
			},
		},
	}
	cntCol := 0
	cntColMani := 0
	for _, i := range inputs {
		var colID int
		q := fmt.Sprintf(`
		INSERT INTO 
			collection(repo,tag) 
		VALUES
			("%s","%s")
		RETURNING id;
		`, i.colRepo, i.colTag)
		rows, err := db.Query(q)
		if err != nil {
			logrus.Error(err)
			continue
		}
		if err != nil {
			logrus.Error(err)
			continue
		}
		if rows.Next() {
			err = rows.Scan(&colID)
			if err != nil {
				logrus.Error(err)
				continue
			}
			rows.Close()
			cntCol++
		}
		for _, m := range i.mani {
			var maniID int
			q = fmt.Sprintf(`SELECT id FROM manifest WHERE image_name="%s"`, m.imageName)
			rows, err := db.Query(q)
			if err != nil {
				logrus.Error(err)
				continue
			}
			if err != nil {
				logrus.Error(err)
				continue
			}
			if rows.Next() {
				err = rows.Scan(&maniID)
				if err != nil {
					logrus.Error(err)
					continue
				}
				rows.Close()
			}
			stmt := fmt.Sprintf(`INSERT INTO col_manifest(collection_id, manifest_id) VALUES (%d, %d)`, colID, maniID)
			query, err := db.Prepare(stmt)
			if err != nil {
				logrus.Error(err)
			}
			res, err := query.Exec()
			if err != nil {
				logrus.Error(err)
			}
			rCnt, _ := res.RowsAffected()
			cntColMani += int(rCnt)
		}
	}
	logrus.Infof("Inserted %d rows into table 'col_manifest' and %d rows in 'collection'", cntColMani, cntCol)
	return
}

func (c Collection) createTable() (err error) {
	err = c.createCollectionTable()
	if err != nil {
		return
	}
	err = c.createCollectionManifestTable()
	if err != nil {
		return
	}
	return
}

func (c *Collection) GetAll() (cols []Collection, err error) {
	q := `
	SELECT
		collection.id AS colID,
		collection.repo AS colRepo,
		collection.tag AS colTag,
		col_manifest.manifest_id AS maniID,
		upstream.reg AS upReg,
		upstream.repo AS upRepo,
		upstream.tag AS upTag
	FROM
		collection
	INNER JOIN col_manifest 
		ON colID = col_manifest.collection_id
	INNER JOIN manifest 
		ON maniID = manifest.id
	INNER JOIN upstream 
		ON manifest.upstream_id = upstream.id
	`
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
	}
	if err != nil {
		logrus.Error(err)
	}
	if rows.Next() {
		cRes := colRes{}
		err = rows.Scan(
			&cRes.ColID,
			&cRes.ColRepo,
			&cRes.ColTag,
			&cRes.ManiID,
			&cRes.UpReg,
			&cRes.UpRepo,
			&cRes.UpTag,
		)
		if err != nil {
			logrus.Error(err)
		}
		rows.Close()
		fmt.Printf("%+v\n", cRes)
	}
	return
}

func (c *Collection) GetCollectionByID(colID int64) (col Collection, err error) {
	q := fmt.Sprintf(`
	SELECT
		collection.id AS colID,
		collection.repo AS colRepo,
		collection.tag AS colTag,
		col_manifest.manifest_id AS maniID,
		upstream.reg AS upReg,
		upstream.repo AS upRepo,
		upstream.tag AS upTag,
		platform.os AS os,
		platform.arch AS arch,
		platform.variant AS variant,
		platform.features AS features,
		profile.name	 AS profile
	FROM
		collection
	INNER JOIN col_manifest 
		ON colID = col_manifest.collection_id
	INNER JOIN manifest 
		ON maniID = manifest.id
	INNER JOIN upstream 
		ON manifest.upstream_id = upstream.id
	INNER JOIN platform 
		ON manifest.platform_id = platform.id
	INNER JOIN platform_filter 
		ON manifest.platform_id = platform_filter.platform_id
	INNER JOIN profile_filter 
		ON platform_filter.filter_id = profile_filter.filter_id
	INNER JOIN profile
		ON profile_filter.profile_id = profile.id
	WHERE
		collection.id = %d
	`, colID)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
	}
	if err != nil {
		logrus.Error(err)
	}
	col = Collection{
		Manifests:     map[string]Manifest{},
		ManifestCount: 0,
	}
	for rows.Next() {
		cRes := colRes{}
		err = rows.Scan(
			&cRes.ColID,
			&cRes.ColRepo,
			&cRes.ColTag,
			&cRes.ManiID,
			&cRes.UpReg,
			&cRes.UpRepo,
			&cRes.UpTag,
			&cRes.OS,
			&cRes.Arch,
			&cRes.Variant,
			&cRes.Features,
			&cRes.Profile,
		)
		if err != nil {
			logrus.Error(err)
		}
		if col.ManifestCount == 0 {
			col.ID = int64(cRes.ColID)
			col.Repo = cRes.ColRepo
			col.Tag = cRes.ColTag
		}
		// Add the manifest to the collection
		m := Manifest{
			ID: int64(cRes.ManiID),
			Upstream: Upstream{
				Reg:  cRes.UpReg,
				Repo: cRes.UpRepo,
				Tag:  cRes.UpTag,
			},
			PlatformSpec: Platform{
				OS:       cRes.OS,
				Arch:     cRes.Arch,
				Variant:  cRes.Variant,
				Features: cRes.Features,
			},
			Profile: Profile{Name: cRes.Profile},
		}
		key := fmt.Sprintf("%s/%s:%s", cRes.UpReg, cRes.UpRepo, cRes.UpTag)
		col.Manifests[key] = m
		col.ManifestCount++
	}
	return
}

func (c *Collection) GetCollectionByRepo(repo string) (cols []Collection, err error) {
	cols = []Collection{}
	q := fmt.Sprintf(`
	SELECT
		collection.id AS colID,
		collection.repo AS colRepo,
		collection.tag AS colTag,
		col_manifest.manifest_id AS maniID,
		upstream.reg AS upReg,
		upstream.repo AS upRepo,
		upstream.tag AS upTag
	FROM
		collection
	INNER JOIN col_manifest 
		ON colID = col_manifest.collection_id
	INNER JOIN manifest 
		ON maniID = manifest.id
	INNER JOIN upstream 
		ON manifest.upstream_id = upstream.id
	WHERE
		collection.repo LIKE '%s%%'
	`, repo)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
	}
	if err != nil {
		logrus.Error(err)
	}
	col := Collection{
		ID: 0,
	}
	for rows.Next() {
		cRes := colRes{}
		err = rows.Scan(
			&cRes.ColID,
			&cRes.ColRepo,
			&cRes.ColTag,
			&cRes.ManiID,
			&cRes.UpReg,
			&cRes.UpRepo,
			&cRes.UpTag,
		)
		if err != nil {
			logrus.Error(err)
		}
		if col.ID != int64(cRes.ColID) {
			if col.ID != 0 {
				// In case it is not 0 it holds the previous collection, thus we add it
				cols = append(cols, col)
			}
			col = Collection{
				ID:            int64(cRes.ColID),
				Repo:          cRes.ColRepo,
				Tag:           cRes.ColTag,
				Manifests:     map[string]Manifest{},
				ManifestCount: 0,
			}
		}
		// Add the manifest to the collection
		m := Manifest{
			ID: int64(cRes.ManiID),
			Upstream: Upstream{
				Reg:  cRes.UpReg,
				Repo: cRes.UpRepo,
				Tag:  cRes.UpTag,
			},
		}
		key := fmt.Sprintf("%s/%s:%s", cRes.UpReg, cRes.UpRepo, cRes.UpTag)
		col.Manifests[key] = m
		col.ManifestCount++
		//fmt.Printf("%+v\n", cRes)
	}
	cols = append(cols, col)

	return
}
