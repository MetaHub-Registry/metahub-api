package types

import (
	"github.com/docker/distribution"
	"github.com/opencontainers/go-digest"
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
