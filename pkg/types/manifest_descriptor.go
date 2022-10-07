package types

import "github.com/docker/distribution/manifest/manifestlist"

type ManifestDescriptor struct {
	Digest    string                    `json:"digest"`
	MediaType string                    `json:"mediaType"`
	Size      int                       `json:"size"`
	Platform  manifestlist.PlatformSpec `json:"platform"`
}
