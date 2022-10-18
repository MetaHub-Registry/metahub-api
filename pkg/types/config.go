package types

import (
	"encoding/json"

	"github.com/docker/distribution"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

type Config struct {
	NeoID     int64  `json:"neo_id"`
	SqlID     int64  `json:"sql_id"`
	Data      []byte `json:"-"`
	Digest    string `json:"digest"`
	Label     string `json:"label"`
	MediaType string `json:"media_type"`
	Size      int64  `json:"size"`
}

func NewConfigFromBytes(data []byte) (res Config, err error) {
	l := logrus.WithFields(logrus.Fields{
		"packrusage": "types/config.go",
		"func":       "NewConfigFromBytes",
	})
	err = json.Unmarshal(data, &res)
	if err != nil {
		l.Error(err)
	}
	res.Data = data
	return
}

func (c Config) ToDescriptor() distribution.Descriptor {
	return distribution.Descriptor{
		Digest:    digest.Digest(c.Digest),
		MediaType: c.MediaType,
		Size:      c.Size,
	}
}
