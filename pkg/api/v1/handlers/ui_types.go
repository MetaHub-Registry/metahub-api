package handlers

import (
	"strings"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

// UIManifest is the response DTO for a single manifest, matching the frontend's Manifest interface.
// Field names use snake_case to align with the React UI expectations.
type UIManifest struct {
	Digest      string            `json:"digest"`
	MediaType   string            `json:"media_type"`
	Platform    UIPlatform        `json:"platform"`
	Size        int64             `json:"size"`
	Registry    string            `json:"registry"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// UIPlatform matches the frontend's platform sub-object, including optional features.
type UIPlatform struct {
	OS           string   `json:"os"`
	Architecture string   `json:"architecture"`
	Variant      string   `json:"variant,omitempty"`
	Features     []string `json:"features,omitempty"`
}

// UICollectionDetail is the response DTO for a single collection, matching the frontend's
// CollectionDetailResponse.collection shape: { name, tag, manifests: Manifest[] }.
type UICollectionDetail struct {
	Name      string       `json:"name"`
	Tag       string       `json:"tag"`
	Manifests []UIManifest `json:"manifests"`
}

// toUIManifest converts a metalib ManifestData into a UIManifest suitable for the frontend.
func toUIManifest(md types.Manifest) UIManifest {
	annotations := md.Annotations.ToMap()
	features := extractFeatures(annotations)
	return UIManifest{
		Digest:    md.Digest,
		MediaType: md.MediaType,
		Platform: UIPlatform{
			OS:           md.Platform.OS,
			Architecture: md.Platform.Arch,
			Variant:      md.Platform.Variant,
			Features:     features,
		},
		Size:        md.Size,
		Registry:    md.Upstream.Reg,
		Annotations: annotations,
	}
}

// extractFeatures scans annotations for keys prefixed with "cpu:" and returns
// those whose value is "true" as a list of feature strings (e.g. ["cpu:skylake"]).
func extractFeatures(annotations map[string]string) []string {
	var features []string
	for k, v := range annotations {
		if strings.HasPrefix(k, "cpu:") && v == "true" {
			features = append(features, k)
		}
	}
	return features
}

// toUICollectionDetail converts a metalib CollectionListing into a UICollectionDetail.
// The manifests map is flattened into a sorted array.
func toUICollectionDetail(col types.CollectionListing) UICollectionDetail {
	manifests := make([]UIManifest, 0, len(col.Manifests))
	for _, md := range col.Manifests {
		manifests = append(manifests, toUIManifest(md))
	}
	return UICollectionDetail{
		Name:      col.Repo,
		Tag:       col.Tag,
		Manifests: manifests,
	}
}
