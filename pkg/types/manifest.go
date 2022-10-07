package types

import (
	"encoding/json"
	"strings"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/opencontainers/go-digest"
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

func (m Manifest) ToBytes() (data []byte, err error) {
	data, err = json.Marshal(m)
	return
}

func (m Manifest) ToOCIManifestDescriptor() (md manifestlist.ManifestDescriptor) {
	feats := []string{}
	for _, f := range strings.Split(m.Platform.Features, ",") {
		if f != "" {
			feats = append(feats, f)
		}
	}
	strings.Split(m.Platform.Features, ",")
	return manifestlist.ManifestDescriptor{
		distribution.Descriptor{
			Digest:    digest.Digest(m.Digest),
			MediaType: m.MediaType,
			Size:      m.Size,
		},
		manifestlist.PlatformSpec{
			Architecture: m.Platform.Arch,
			OS:           m.Platform.OS,
			Variant:      m.Platform.Variant,
			Features:     feats,
		},
	}
}
