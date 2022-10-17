package sqlite

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/types"
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

type maniRow struct {
	colID           int64
	maniID          int64
	maniDigest      string
	upstreamReg     string
	upstreamRepo    string
	upstreamTag     string
	os              string
	arch            string
	variant         string
	features        string
	configDigest    string
	configMediaType string
	configSize      int
	profileName     string
}

func (m *Manifest) GetByCollectionId(colId int64) (manis []Manifest, err error) {
	manis = []Manifest{}
	q := fmt.Sprintf(`SELECT * FROM main.manifests_view WHERE colID =%d;`, colId)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
	}
	if err != nil {
		logrus.Error(err)
	}
	for rows.Next() {
		maniRes := maniRow{}
		err = rows.Scan(
			&maniRes.colID,
			&maniRes.maniID,
			&maniRes.maniDigest,
			&maniRes.upstreamReg,
			&maniRes.upstreamRepo,
			&maniRes.upstreamTag,
			&maniRes.os,
			&maniRes.arch,
			&maniRes.variant,
			&maniRes.features,
			&maniRes.configDigest,
			&maniRes.configMediaType,
			&maniRes.configSize,
			&maniRes.profileName,
		)
		if err != nil {
			logrus.Error(err)
		}
		imgName := fmt.Sprintf("%s/%s:%s", maniRes.upstreamReg, maniRes.upstreamRepo, maniRes.upstreamTag)
		mani := Manifest{
			ID:        maniRes.maniID,
			Digest:    maniRes.maniDigest,
			ImageName: imgName,
			PlatformSpec: Platform{
				OS:       maniRes.os,
				Arch:     maniRes.arch,
				Variant:  maniRes.variant,
				Features: maniRes.features,
			},
			Upstream: Upstream{
				Reg:  maniRes.upstreamReg,
				Repo: maniRes.upstreamRepo,
				Tag:  maniRes.upstreamTag,
			},
			Config: Config{
				Digest:    maniRes.configDigest,
				MediaType: maniRes.configMediaType,
				Size:      maniRes.configSize,
			},
			Profile: Profile{
				Name: maniRes.profileName,
			},
		}
		manis = append(manis, mani)
	}
	return
}

func (m Manifest) CreateViews() (err error) {
	err = m.CreateManifestView()
	return
}

func (m Manifest) CreateManifestView() (err error) {
	q := `CREATE VIEW IF NOT EXISTS manifests_view AS
	SELECT
	    collections.id AS colID,
		manifests.id AS maniID,
		manifests.digest AS maniDigest,
		upstreams.reg AS upstreamReg,
		upstreams.repo AS upstreamRepo,
		upstreams.tag AS upstreamTag,
		platforms.os AS os,
		platforms.arch AS arch,
		platforms.variant AS variant,
		platforms.features AS features,
		configs.digest AS configDigest,
		configs.media_type AS configMediaType,
		configs.size AS configSize,
		profiles.name AS profile
	FROM
		collections
	INNER JOIN collection_xtable ON collections.id = collection_xtable.collection_id
	INNER JOIN manifestlists ON collection_xtable.manifestlist_id = manifestlists.id
	INNER JOIN manifestlist_xtable ON manifestlists.id = manifestlist_xtable.manifestlist_id
	INNER JOIN manifests ON manifestlist_xtable.manifest_id = manifests.id
	INNER JOIN upstreams ON manifests.upstream_id = upstreams.id
	INNER JOIN platforms ON manifests.platform_id = platforms.id
	INNER JOIN platform_filter ON platforms.id = platform_filter.platform_id
	INNER JOIN profile_filter ON platform_filter.filter_id = profile_filter.filter_id
	INNER JOIN profiles ON profiles.id = profile_filter.profile_id
	INNER JOIN configs ON manifests.config_id = configs.id`
	_, err = db.Exec(q)
	return
}

func (m Manifest) CreateTables() (err error) {
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
		"media_type" TEXT,
		"size" INT,
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
	return
}

func (m Manifest) Insert(i *types.Manifest) (err error) {
	q := `INSERT INTO manifest (digest, media_type, size, platform_id, upstream_id, config_id) VALUES (?, ?, ?, ?, ?, ?)`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	res, err := stmt.Exec(i.Digest, i.MediaType, i.Size, i.Platform.SqlID, i.Upstream.SqlID, i.Config.SqlID)
	if err != nil {
		err = errors.Wrap(err, "failed to insert manifest")
		logrus.Error(err)
		return
	}
	i.SqlID, err = res.LastInsertId()
	return
}
