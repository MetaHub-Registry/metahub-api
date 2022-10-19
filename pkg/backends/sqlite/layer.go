package sqlite

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/types"
)

type Layer struct {
	ID        int64  `json:"id"`
	Digest    string `json:"digest"`
	Label     string `json:"label"`
	MediaType string `json:"media_type"`
	Size      int    `json:"size"`
}

func (l Layer) LayerGetByDigest(dig string) (layer types.Layer, found bool, err error) {
	// Already exists?
	q := fmt.Sprintf(`SELECT digest, media_type, size FROM layers WHERE digest="%s"`, dig)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
	}
	type Res struct {
		Digest    string
		MediaType string
		Size      int64
	}
	for rows.Next() {
		if found {
			logrus.Errorf("found more than one layer with digest: %s", dig)
			found = false
			return
		}
		r := Res{}
		err = rows.Scan(
			&r.Digest,
			&r.MediaType,
			&r.Size,
		)
		if err != nil {
			logrus.Error(err)
			return
		}
		layer = types.Layer{
			Digest:    r.Digest,
			MediaType: r.MediaType,
			Size:      r.Size,
		}
		found = true
	}
	return
}
