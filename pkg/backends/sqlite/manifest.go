package sqlite

import (
	"fmt"

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
	maniListID      int64
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
	q := `SELECT DISTINCT colID, maniID, maniDigest, upstreamReg, upstreamRepo, upstreamTag, os, arch, variant, features`
	q += `,configDigest, configMediaType, configSize, profile`
	q += fmt.Sprintf(` FROM main.manifests_view WHERE colID=%d;`, colId)
	//logrus.Error(q)
	rows, err := db.Query(q)
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

func (m *Manifest) GetTypesManifestByRequest(req types.Request) (res types.Manifest, found bool, err error) {
	q := `SELECT DISTINCT maniID,maniDigest,os,arch,variant,features,configDigest,configMediaType,configSize,`
	q += `upstreamReg,upstreamRepo,upstreamTag FROM main.manifests_view`
	q += fmt.Sprintf(` WHERE maniDigest="%s";`, req.Digest)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
	}
	for rows.Next() {
		maniRes := maniRow{}
		err = rows.Scan(
			&maniRes.maniID,
			&maniRes.maniDigest,
			&maniRes.os,
			&maniRes.arch,
			&maniRes.variant,
			&maniRes.features,
			&maniRes.configDigest,
			&maniRes.configMediaType,
			&maniRes.configSize,
			&maniRes.upstreamReg,
			&maniRes.upstreamRepo,
			&maniRes.upstreamTag,
		)
		if err != nil {
			logrus.Error(err)
			return
		}
		found = true
		res = types.Manifest{
			SqlID:     maniRes.maniID,
			Digest:    maniRes.maniDigest,
			MediaType: "application/vnd.docker.distribution.manifest.v2+json",
			Platform: types.Platform{
				OS:       maniRes.os,
				Arch:     maniRes.arch,
				Variant:  maniRes.variant,
				Features: maniRes.features,
			},
			Config: types.Config{
				Digest:    maniRes.configDigest,
				MediaType: maniRes.configMediaType,
				Size:      int64(maniRes.configSize),
			},
			Upstream: types.Upstream{
				Reg:  maniRes.upstreamReg,
				Repo: maniRes.upstreamRepo,
				Tag:  maniRes.upstreamTag,
			},
			Layers: []types.Layer{},
		}
	}
	return
}

func (m *Manifest) TypesManifest_GetByManifestListID(mlID int64) (manis []types.Manifest, err error) {
	manis = []types.Manifest{}
	q := `SELECT maniID,maniDigest,maniListID,os,arch,variant,features,configDigest,configMediaType,configSize,`
	q += `upstreamReg,upstreamRepo,upstreamTag	FROM main.manifests_view`
	q += fmt.Sprintf(` WHERE maniListID=%d;`, mlID)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
	}
	for rows.Next() {
		maniRes := maniRow{}
		err = rows.Scan(
			&maniRes.maniID,
			&maniRes.maniDigest,
			&maniRes.maniListID,
			&maniRes.os,
			&maniRes.arch,
			&maniRes.variant,
			&maniRes.features,
			&maniRes.configDigest,
			&maniRes.configMediaType,
			&maniRes.configSize,
			&maniRes.upstreamReg,
			&maniRes.upstreamRepo,
			&maniRes.upstreamTag,
		)
		if err != nil {
			logrus.Error(err)
		}
		mani := types.Manifest{
			SqlID:     maniRes.maniID,
			Digest:    maniRes.maniDigest,
			MediaType: "application/vnd.docker.distribution.manifest.v2+json",
			Platform: types.Platform{
				OS:       maniRes.os,
				Arch:     maniRes.arch,
				Variant:  maniRes.variant,
				Features: maniRes.features,
			},
			Config: types.Config{
				Digest:    maniRes.configDigest,
				MediaType: maniRes.configMediaType,
				Size:      int64(maniRes.configSize),
			},
			Upstream: types.Upstream{
				Reg:  maniRes.upstreamReg,
				Repo: maniRes.upstreamRepo,
				Tag:  maniRes.upstreamTag,
			},
			Layers: []types.Layer{},
		}
		manis = append(manis, mani)
	}
	return
}
