package types

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/docker/distribution"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

type Layer struct {
	NeoID     int64  `json:"neo_id"`
	SqlID     int64  `json:"sql_id"`
	Data      []byte `json:"-"`
	Digest    string `json:"digest"`
	MediaType string `json:"media_type"`
	Size      int64  `json:"size"`
}

func (l Layer) ToDescriptor() distribution.Descriptor {
	return distribution.Descriptor{
		Digest:    digest.Digest(l.Digest),
		MediaType: l.MediaType,
		Size:      l.Size,
	}
}

func (l Layer) WriteWWWHeader(w http.ResponseWriter) {
	// TODO: Not sure if that is stable over time. We might need to
	b := l.Data
	size := len(b)
	w.Header().Set("Content-Type", l.MediaType)
	w.Header().Set("Content-Length", fmt.Sprint(size))
	w.Header().Set("Docker-Content-Digest", l.Digest)
	w.Header().Set("Etag", l.Digest)
}

func (l Layer) WriteWWWBody(w http.ResponseWriter) {
	w.Write(l.Data)
}

func NewLayerFromBytes(data []byte) (res Layer, err error) {
	l := logrus.WithFields(logrus.Fields{
		"packrusage": "types/config.go",
		"func":       "NewLayerFromBytes",
	})
	err = json.Unmarshal(data, &res)
	if err != nil {
		l.Error(err)
	}
	res.Data = data
	return
}
