package sqlite

import (
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

func (c *Collection) GetAll() (cols []Collection, err error) {
	q := `SELECT * FROM main.v_get_collections`
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

func (c *Collection) GetCollectionByID(colID int64) (col Collection, err error) {
	q := fmt.Sprintf(`SELECT * FROM main.v_get_collection WHERE colID = %d`, colID)
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
	q := fmt.Sprintf(`SELECT * FROM main.v_get_collections WHERE colRepo LIKE '%s%%';`, repo)
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
		upstream.tag AS upstreamTag
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
