package types

import (
	"encoding/json"

	"github.com/sirupsen/logrus"
)

type Manifest struct {
	NeoID     int64    `json:"neo_id"`
	SqlID     int64    `json:"sql_id"`
	Digest    string   `json:"digest"`
	Data      []byte   `json:"data,omitempty"`
	MediaType string   `json:"media_type"`
	Size      int64    `json:"size"`
	Platform  Platform `json:"platform"`
	Upstream  Upstream `json:"upstream"`
	Config    Config   `json:"config"`
	Layers    []Layer  `json:"layers"`
}

func NewManifestFromBytes(data []byte) (mani Manifest, err error) {
	l := logrus.WithFields(logrus.Fields{
		"packrusage": "types/manifest.go",
		"func":       "NewManifestFromBytes",
	})
	err = json.Unmarshal(data, &mani)
	if err != nil {
		l.Error(err)
	}
	mani.Data = data
	return
}

func (m Manifest) ToBytes() (data []byte, err error) {
	data, err = json.Marshal(m)
	return
}
