package types

import (
	"encoding/json"
	"fmt"
	"net/http"
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

func (m Manifest) ToManifestDescriptor() (md ManifestDescriptor) {
	feats := []string{}
	for _, f := range strings.Split(m.Platform.Features, ",") {
		if f != "" {
			feats = append(feats, f)
		}
	}
	strings.Split(m.Platform.Features, ",")
	return ManifestDescriptor{
		Digest:    m.Digest,
		MediaType: m.MediaType,
		Size:      int(m.Size),
		Platform: manifestlist.PlatformSpec{
			Architecture: m.Platform.Arch,
			OS:           m.Platform.OS,
			Variant:      m.Platform.Variant,
			Features:     feats,
		},
	}
}

type OCIManifest struct {
	// SchemaVersion is the image manifest schema that this image follows
	SchemaVersion int `json:"schemaVersion"`
	// MediaType is the media type of this schema.
	MediaType string `json:"mediaType,omitempty"`
	// Config references the image configuration as a blob.
	Config distribution.Descriptor `json:"config"`
	// Layers lists descriptors for the layers referenced by the
	// configuration.
	Layers []distribution.Descriptor `json:"layers"`
}

func (m Manifest) ToOCIBytes() (b []byte) {
	oci := OCIManifest{
		SchemaVersion: 2,
		MediaType:     m.MediaType,
		Config:        m.Config.ToDescriptor(),
		Layers:        []distribution.Descriptor{},
	}
	for _, l := range m.Layers {
		oci.Layers = append(oci.Layers, l.ToDescriptor())
	}
	b, _ = json.Marshal(oci)
	return
}

func (m Manifest) WriteWWWHeader(w http.ResponseWriter) {
	size := len(m.Data)
	w.Header().Set("Content-Type", m.MediaType)
	w.Header().Set("Content-Length", fmt.Sprint(size))
	w.Header().Set("Docker-Content-Digest", m.Digest)
	w.Header().Set("Etag", m.Digest)
}

func (m Manifest) WriteWWWBody(w http.ResponseWriter) {
	b := m.Data
	w.Write(b)
}
