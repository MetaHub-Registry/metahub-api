package sqlite

import (
	"database/sql"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/types"
)

type Collection struct {
	ID            int64               `json:"id"`
	Repo          string              `json:"repo"`
	Tag           string              `json:"tag"`
	Manifests     map[string]Manifest `json:"manifests"`
	ManifestCount int64               `json:"manifest_count"`
}

type maniRes struct {
	ConfigDigest string `json:"config_digest"`
	ConfigID     int64  `json:"config_id"`
	ManiID       int64  `json:"mani_id"`
	ManiDigest   string `json:"mani_digest"`
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	Variant      string `json:"variant,omitempty"`
	Features     string `json:"features,omitempty"`
	UpReg        string `json:"up_reg"`
	UpRepo       string `json:"up_repo"`
	UpTag        string `json:"up_tag"`
}

type mlRes struct {
	ColID    int    `json:"collection_id"`
	MlID     int    `json:"manifest_list_id"`
	ColRepo  string `json:"collection_repo"`
	ColTag   string `json:"collection_tag"`
	MLDigest string `json:"manifest_list_digest"`
	MlDigest string `json:"manifest_digest"`
	UpReg    string `json:"upstream_reg"`
	UpRepo   string `json:"upstream_repo"`
	UpTag    string `json:"upstream_tag"`
	Profile  string `json:"profile"`
}

type cfgRes struct {
	ConfigDigest    string
	ConfigID        int64
	ConfigSize      int64
	ConfigMediaType string
}

type colRes struct {
	ColID      int    `json:"collection_id"`
	ColRepo    string `json:"collection_repo"`
	ColTag     string `json:"collection_tag"`
	ManiID     int    `json:"manifest_id"`
	ManiDigest string `json:"manifest_digest"`
	MlDigest   string `json:"manifestlist_digest"`
	UpReg      string `json:"upstream_reg"`
	UpRepo     string `json:"upstream_repo"`
	UpTag      string `json:"upstream_tag"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	Variant    string `json:"variant"`
	Features   string `json:"features"`
	Profile    string `json:"profile"`
}

func (c *Collection) GetAll() (cols []Collection, err error) {
	q := `SELECT DISTINCT colID, colRepo, colTag, maniID, upstreamReg, upstreamRepo, upstreamTag, os, arch, variant, features, profile FROM main.v_get_collections`
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
		return
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
			&cRes.OS,
			&cRes.Arch,
			&cRes.Variant,
			&cRes.Features,
			&cRes.Profile,
		)
		if err != nil {
			logrus.Error(err)
			return
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
		cols = append(cols, col)
	}

	return
}

func createCol(row *sql.Rows) (col Collection, err error) {
	cRes := colRes{}
	err = row.Scan(
		&cRes.ColID,
		&cRes.ColRepo,
		&cRes.ColTag,
		&cRes.ManiID,
		&cRes.MlDigest,
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
	return
}

func (c *Collection) GetCollectionByID(colID int64) (col Collection, err error) {
	q := fmt.Sprintf(`SELECT * FROM main.v_get_collections WHERE colID = %d`, colID)
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
			&cRes.MlDigest,
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

func (c *Collection) GetTypesManifestByCollectionID(colID int64) (manis []types.Manifest, err error) {
	q := fmt.Sprintf(`SELECT * FROM main.v_get_manifest WHERE colID=%d;`, colID)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
	}
	manis = []types.Manifest{}
	for rows.Next() {
		res := maniRes{}
		err = rows.Scan(
			&res.ConfigDigest,
			&res.ConfigID,
			&res.ManiDigest,
			&res.ManiID,
			&res.OS,
			&res.Arch,
			&res.Variant,
			&res.Features,
			&res.UpReg,
			&res.UpRepo,
			&res.UpTag,
		)
		plat := types.Platform{
			OS:       res.OS,
			Arch:     res.Arch,
			Variant:  res.Variant,
			Features: res.Features,
		}
		// Get the config
		cfg, err := c.GetConfigByConfigID(int64(res.ConfigID))
		if err != nil {
			logrus.Error(err)
			continue
		}

		// Layers
		layers := []types.Layer{}
		if err != nil {
			logrus.Error(err)
			continue
		}
		// TODO: Get Layers
		m := types.Manifest{
			Platform: plat,
			Upstream: types.Upstream{
				Reg:  res.UpReg,
				Repo: res.UpRepo,
				Tag:  res.UpTag,
			},
			Config: cfg,
			Layers: layers,
		}
		manis = append(manis, m)
	}
	return
}

func (c Collection) GetConfigByConfigID(cfgID int64) (cfg types.Config, err error) {
	q := `SELECT 
		configs.digest AS configDigest,
		configs.id AS configID,
		configs.size AS configSize,
		configs.mediaType AS configMediaType
	FROM configs WHERE configs.id=%d;`
	query := fmt.Sprintf(q, cfgID)
	rows, err := db.Query(query)
	if err != nil {
		logrus.Error(err)
	}
	cfg = types.Config{
		SqlID: cfgID,
	}
	for rows.Next() {
		res := cfgRes{}
		err = rows.Scan(
			&res.ConfigDigest,
			&res.ConfigID,
			&res.ConfigSize,
			&res.ConfigMediaType,
		)
		cfg.Digest = res.ConfigDigest
		cfg.Size = res.ConfigSize
		cfg.MediaType = res.ConfigMediaType
	}
	return
}

func (c *Collection) GetCollectionsByRepoLike(repo string) (cols []Collection, err error) {
	cols = []Collection{}
	q := fmt.Sprintf(`SELECT * FROM main.v_get_collections WHERE colRepo LIKE '%s%%';`, repo)
	rows, err := db.Query(q)
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
			&cRes.ManiDigest,
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

func (c *Collection) CreateViews() (err error) {
	err = c.CreateCollectionsView()
	if err != nil {
		logrus.Error(err)
		return
	}
	err = c.CreateCollectionView()
	if err != nil {
		logrus.Error(err)
		return
	}
	return
}

func (c *Collection) CreateCollectionsView() (err error) {
	q := `CREATE VIEW IF NOT EXISTS v_get_collections AS
	SELECT
		collection.id AS colID,
		collection.repo AS colRepo,
		collection.tag AS colTag,
		manifest.id AS maniID,
		upstream.reg AS upstreamReg,
		upstream.repo AS upstreamRepo,
		upstream.tag AS upstreamTag,
		platform.os AS os,
		platform.arch AS arch,
		platform.variant AS variant,
		platform.features AS features,
		profile.name	 AS profile
	FROM
		collection
		INNER JOIN collection_xtable ON collection.id = collection_xtable.collection_id
		INNER JOIN manifestlist ON collection_xtable.manifestlist_id = manifestlist.id
		INNER JOIN manifestlist_xtable ON manifestlist.id = manifestlist_xtable.manifestlist_id
		INNER JOIN manifest ON manifestlist_xtable.manifest_id = manifest.id
		INNER JOIN upstream ON manifest.upstream_id = upstream.id
		INNER JOIN platform ON manifest.platform_id = platform.id
		INNER JOIN platform_filter ON platform.id = platform_filter.platform_id
		INNER JOIN profile_filter ON platform_filter.filter_id = profile_filter.filter_id
		INNER JOIN profile ON profile.id = profile_filter.profile_id`
	_, err = db.Exec(q)
	return
}

func (c *Collection) CreateCollectionView() (err error) {
	q := `CREATE VIEW IF NOT EXISTS v_get_collection AS
	SELECT
		collection.id AS colID,
		collection.repo AS colRepo,
		collection.tag AS colTag,
		manifest.id AS maniID,
		upstream.reg AS upstreamReg,
		upstream.repo AS upstreamRepo,
		upstream.tag AS upstreamTag,
		platform.os AS os,
		platform.arch AS arch,
		platform.variant AS variant,
		platform.features AS features,
		profile.name	 AS profile
	FROM collection
	INNER JOIN collection_xtable
		ON collection.id = collection_xtable.collection_id
	INNER JOIN manifestlist
		ON collection_xtable.manifestlist_id = manifestlist.id
	INNER JOIN manifestlist_xtable
		ON manifestlist.id = manifestlist_xtable.manifestlist_id
	INNER JOIN manifest
		ON manifestlist_xtable.manifest_id = manifest.id
	INNER JOIN upstream
		ON manifest.upstream_id = upstream.id
	INNER JOIN platform
		ON manifest.platform_id = platform.id
	INNER JOIN platform_filter
		ON platform.id = platform_filter.platform_id
	INNER JOIN profile_filter
		ON platform_filter.filter_id = profile_filter.filter_id
	INNER JOIN profile
		ON profile.id = profile_filter.profile_id`
	_, err = db.Exec(q)
	return
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

func (c Collection) createCollectionXTable() (err error) {
	tableName := "collection_xtable"
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
		"manifestlist_id" INT,
		FOREIGN KEY (collection_id)
			REFERENCES collection (id)
		FOREIGN KEY (manifestlist_id)
			REFERENCES manifestlist (id)
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

func (c Collection) CreateTables() (err error) {
	err = c.createCollectionTable()
	if err != nil {
		return
	}
	err = c.createCollectionXTable()
	if err != nil {
		return
	}
	return
}

func (c Collection) Insert(i *types.Collection) (err error) {
	q := `INSERT INTO collection (repo, tag) VALUES (?, ?)`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	res, err := stmt.Exec(i.Repo, i.Tag)
	if err != nil {
		logrus.Error(err)
		return
	}
	i.SqlID, err = res.LastInsertId()
	return
}

func (c Collection) InsertXTable(colID, mlID int64) (err error) {
	q := `INSERT INTO collection_xtable (collection_id, manifestlist_id) VALUES (?, ?)`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	_, err = stmt.Exec(colID, mlID)
	if err != nil {
		logrus.Error(err)
		return
	}
	return
}
