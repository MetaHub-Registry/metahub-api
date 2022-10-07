package types

import "github.com/opencontainers/go-digest"

// Blob describes a
type BlobMeta struct {
	Digest    digest.Digest `json:"digest"`
	MediaType string        `json:"mediaType"`
	Size      int64         `json:"size"`
}
