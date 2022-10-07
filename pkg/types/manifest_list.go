package types

import (
	"encoding/json"

	"github.com/docker/distribution/manifest/manifestlist"
)

type ManifestList struct {
	NeoID         int64      `json:"neo_id"`
	SqlID         int64      `json:"sql_id"`
	Digest        string     `json:"digest"`
	SchemaVersion int        `json:"schema_version"`
	MediaType     string     `json:"media_type"`
	Manifests     []Manifest `json:"manifests"`
}

type OCIManifestList struct {
	SchemaVersion int                               `json:"schemaVersion"`
	MediaType     string                            `json:"mediaType"`
	Manifests     []manifestlist.ManifestDescriptor `json:"manifests"`
}

func (ml ManifestList) ToOCIBytes() (b []byte) {
	oci := OCIManifestList{
		SchemaVersion: ml.SchemaVersion,
		MediaType:     ml.MediaType,
		Manifests:     []manifestlist.ManifestDescriptor{},
	}
	for _, m := range ml.Manifests {
		oci.Manifests = append(oci.Manifests, m.ToOCIManifestDescriptor())
	}
	b, _ = json.Marshal(oci)
	return
}
