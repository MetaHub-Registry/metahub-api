package handlers

import (
	"strings"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

// UIDescriptor is the shared shape for config and layer blob descriptors
// returned to the UI. It mirrors the OCI descriptor but uses snake_case
// JSON.
type UIDescriptor struct {
	Digest    string `json:"digest"`
	MediaType string `json:"media_type"`
	Size      int64  `json:"size"`
}

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
	// CPUTarget is the structured CPU optimization parsed from the
	// org.supercontainers.hardware.cpu.target* annotations, when present.
	// Lets the frontend distinguish otherwise-identical platform rows
	// (e.g. a zen2-optimized amd64 image from a plain amd64 one).
	CPUTarget *UICPUTarget `json:"cpu_target,omitempty"`
	// Config is the image config blob descriptor when known.
	Config *UIDescriptor `json:"config,omitempty"`
	// Layers lists the layer blob descriptors in layer order.
	Layers []UIDescriptor `json:"layers,omitempty"`
}

// UICPUTarget is the structured CPU optimization for a manifest, parsed
// from the org.supercontainers.hardware.cpu.target* annotations.
// Kind is the discriminator ("platform", "microarch", or "generic");
// Microarch / Generic carry the specific value when applicable.
type UICPUTarget struct {
	Kind      string `json:"kind,omitempty"`
	Microarch string `json:"microarch,omitempty"`
	Generic   string `json:"generic,omitempty"`
}

// UIManipuCommand represents a CMD or ENTRYPOINT mutation.
type UIManipuCommand struct {
	Op    string   `json:"op"`
	Value []string `json:"value"`
}

// UIManipuLayer is a single layer referenced by a manipulation's default
// or variant block. The image URL is best-effort: populated from the
// upstream when known, otherwise empty.
//
// `Select` carries the optional Python-style slice expression (e.g.
// "[1:]") that trims which layers from the source image actually get
// appended at apply time. Empty means "all layers".
type UIManipuLayer struct {
	Image    string `json:"image,omitempty"`
	Rule     string `json:"rule,omitempty"`
	Upstream string `json:"upstream,omitempty"`
	Select   string `json:"select,omitempty"`
}

// UIManipuConfig mirrors metalib's extended manipulation block.
type UIManipuConfig struct {
	Env        map[string]string `json:"env,omitempty"`
	Workdir    string            `json:"workdir,omitempty"`
	User       string            `json:"user,omitempty"`
	Volumes    []string          `json:"volumes,omitempty"`
	Entrypoint *UIManipuCommand  `json:"entrypoint,omitempty"`
	Cmd        *UIManipuCommand  `json:"cmd,omitempty"`
	Layers     []UIManipuLayer   `json:"layers,omitempty"`
}

// UIManipuVariant binds a rule-expression string to a config block.
type UIManipuVariant struct {
	Rule   string         `json:"rule"`
	Config UIManipuConfig `json:"config"`
}

// UIManipulation is the response DTO for a single manipulation linked to
// a profile. `Default` and `Variants` carry the new extended schema;
// `Layers` is populated for legacy-form manipulations that predate the
// extended schema.
type UIManipulation struct {
	ID       int64             `json:"id"`
	Label    string            `json:"label"`
	Default  *UIManipuConfig   `json:"default,omitempty"`
	Variants []UIManipuVariant `json:"variants,omitempty"`
	Layers   []UIManipuLayer   `json:"layers,omitempty"`
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
	Manipulations   []UIManipulation  `json:"manipulations"`
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

	out := UIManifest{
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
		CPUTarget:   cpuTargetFromAnnotations(annotations),
	}
	if md.Config.Digest != "" {
		out.Config = &UIDescriptor{
			Digest:    md.Config.Digest,
			MediaType: md.Config.MediaType,
			Size:      md.Config.Size,
		}
	}
	if len(md.Layers) > 0 {
		out.Layers = make([]UIDescriptor, 0, len(md.Layers))
		for _, lyr := range md.Layers {
			out.Layers = append(out.Layers, UIDescriptor{
				Digest:    lyr.Digest,
				MediaType: lyr.MediaType,
				Size:      lyr.Size,
			})
		}
	}
	return out
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

const (
	annoCPUTarget          = "org.supercontainers.hardware.cpu.target"
	annoCPUTargetMicroarch = "org.supercontainers.hardware.cpu.target.microarch"
	annoCPUTargetGeneric   = "org.supercontainers.hardware.cpu.target.generic"
)

// cpuTargetFromAnnotations parses the org.supercontainers.hardware.cpu.target*
// annotations into a structured CPU target. It returns nil when no
// cpu.target annotation is present, so the DTO field is omitted for
// manifests that carry no CPU optimization metadata.
func cpuTargetFromAnnotations(annotations map[string]string) *UICPUTarget {
	kind := annotations[annoCPUTarget]
	microarch := annotations[annoCPUTargetMicroarch]
	generic := annotations[annoCPUTargetGeneric]
	if kind == "" && microarch == "" && generic == "" {
		return nil
	}
	return &UICPUTarget{Kind: kind, Microarch: microarch, Generic: generic}
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
