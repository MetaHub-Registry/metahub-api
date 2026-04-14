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
	ContentSize int64             `json:"content_size,omitempty"`
	Registry    string            `json:"registry"`
	Upstream    string            `json:"upstream,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Profiles    []string          `json:"profiles"`
}

// ProfileDetailResponse is the response DTO for GET /api/v1/profiles/{label}.
type ProfileDetailResponse struct {
	ID              int64             `json:"id"`
	Name            string            `json:"name"`
	Tag             string            `json:"tag"`
	Label           string            `json:"label"`
	Rules           []ProfileRuleResp `json:"rules"`
	CollectionCount int64             `json:"collection_count"`
	ManifestCount   int64             `json:"manifest_count"`
}

// ProfileRuleResp is the response DTO for a single profile rule.
type ProfileRuleResp struct {
	ID    int64  `json:"id"`
	Type  string `json:"type"`
	Op    string `json:"op"`
	Key   string `json:"key"`
	Value string `json:"value"`
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

	// Build the full upstream URL when repo is known.
	// Prefer the human-readable tag form over the digest form.
	var upstream string
	if md.Upstream.Repo != "" {
		if md.Upstream.Tag != "" {
			upstream = md.Upstream.Reg + "/" + md.Upstream.Repo + ":" + md.Upstream.Tag
		} else {
			upstream = md.Upstream.String()
		}
	}

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
		ContentSize: md.ContentSize,
		Registry:    md.Upstream.Reg,
		Upstream:    upstream,
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
