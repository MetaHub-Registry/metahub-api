package sqlite

import (
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
	q := fmt.Sprintf(`SELECT * FROM main.manifest_view WHERE colID =%d;`, colId)
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
	q := `CREATE VIEW IF NOT EXISTS manifest_view AS
	SELECT
		manifest.id AS maniID,
		manifest.digest AS maniDigest,
		upstream.reg AS upstreamReg,
		upstream.repo AS upstreamRepo,
		upstream.tag AS upstreamTag,
		platform.os AS os,
		platform.arch AS arch,
		platform.variant AS variant,
		platform.features AS features,
		config.digest AS configDigest,
		config.media_type AS configMediaType,
		config.size AS configSize,
		profile.name AS profile
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
