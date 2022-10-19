package sqlite

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/types"
)

// ManifestList serve a collection for a certain profile.
type ManifestList struct {
	ID     int64  `json:"id"`
	Digest string `json:"digest"`
	// key of the map matches the key of the map in Platforms to construct the platformObject indexes of the ManifestList
	Manifests map[string]Manifest `json:"manifests"`
	Platforms map[string]Platform `json:"platform"`
}

func (ml ManifestList) GetTypesManifestListByRequest(req types.Request) (res types.ManifestList, found bool, err error) {
	q := fmt.Sprintf(`SELECT DISTINCT colID,colRepo,colTag,manifestlistDigest,manifestlistID FROM v_get_manifestlist WHERE colRepo="%s" AND colTag="%s" AND profileName="%s"`, req.Repo, req.Tag, req.Profile)
	if req.Tag == "" {
		q = fmt.Sprintf(`SELECT DISTINCT colID,colRepo,colTag,manifestlistDigest,manifestlistID FROM v_get_manifestlist WHERE colRepo="%s" AND manifestlistDigest="%s" AND profileName="%s"`, req.Repo, req.Digest, req.Profile)
	}
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
	}
	// BootStrap the ML
	res = types.ManifestList{
		Digest:        "",
		SchemaVersion: 2,
		MediaType:     "application/vnd.docker.distribution.manifest.list.v2+json",
		Manifests:     []types.Manifest{},
	}
	for rows.Next() {
		found = true
		r := mlRes{}
		err = rows.Scan(
			&r.ColID,
			&r.ColRepo,
			&r.ColTag,
			&r.MlDigest,
			&r.MlID,
		)
		if err != nil {
			logrus.Error(err)
			continue
		}
		res.SqlID = int64(r.MlID)
		if res.Digest == "" {
			// First row
			res.Digest = r.MlDigest
		}
	}
	return
}
