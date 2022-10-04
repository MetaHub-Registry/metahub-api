package types

import "github.com/opencontainers/go-digest"

// Blob describes a
type BlobMeta struct {
	Digest digest.Digest
	// MediaType describe the type of the content. All text based formats are
	// encoded as utf-8.
	MediaType string

	// Size in bytes of content.
	Size int64
}
