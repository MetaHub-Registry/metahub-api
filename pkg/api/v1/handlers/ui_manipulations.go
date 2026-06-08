package handlers

import "github.com/metahub-registry/metahub-core/pkg/types"

// toUIManipulations converts a slice of `types.Manipulation` into the
// DTO shape returned by `GET /api/v1/profiles/{label}`. A nil or empty
// input slice yields an empty (non-nil) output slice so the JSON payload
// is stable.
func toUIManipulations(mps []types.Manipulation) []UIManipulation {
	out := make([]UIManipulation, 0, len(mps))
	for _, mp := range mps {
		out = append(out, toUIManipulation(mp))
	}
	return out
}

// toUIManipulation converts a single `types.Manipulation` to its DTO.
// `Default` is omitted when zero; `Variants` is always a slice (may be
// empty); `Layers` is populated from the legacy flat form when present.
func toUIManipulation(mp types.Manipulation) UIManipulation {
	ui := UIManipulation{
		ID:    mp.ID,
		Label: mp.Label,
	}
	if !mp.Default.IsZero() {
		cfg := toUIManipuConfig(mp.Default)
		ui.Default = &cfg
	}
	if len(mp.Variants) > 0 {
		ui.Variants = make([]UIManipuVariant, 0, len(mp.Variants))
		for _, v := range mp.Variants {
			ui.Variants = append(ui.Variants, UIManipuVariant{
				Rule:   v.Rule,
				Config: toUIManipuConfig(v.Config),
			})
		}
	}
	// Legacy flat layers — populated when the manipulation was created
	// before the extended schema was introduced. Prefer the legacy shape
	// only when Default is zero (there are no modern variants either in
	// that case).
	if mp.Default.IsZero() && len(mp.Variants) == 0 && len(mp.ManipuLayers) > 0 {
		ui.Layers = toUIManipuLayers(mp.ManipuLayers)
	}
	return ui
}

// toUIManipuConfig converts a `types.ManipuConfig` to its DTO.
func toUIManipuConfig(cfg types.ManipuConfig) UIManipuConfig {
	ui := UIManipuConfig{
		Env:        cfg.Env,
		Workdir:    cfg.Workdir,
		User:       cfg.User,
		Volumes:    cfg.Volumes,
		Entrypoint: toUIManipuCommand(cfg.Entrypoint),
		Cmd:        toUIManipuCommand(cfg.Cmd),
		Layers:     toUIManipuLayers(cfg.Layers),
	}
	return ui
}

// toUIManipuCommand converts a `*types.ManipuCommand` to its DTO pointer.
// Returns nil for nil or zero-valued commands.
func toUIManipuCommand(cmd *types.ManipuCommand) *UIManipuCommand {
	if cmd == nil || cmd.IsZero() {
		return nil
	}
	value := make([]string, len(cmd.Value))
	copy(value, cmd.Value)
	return &UIManipuCommand{Op: cmd.Op, Value: value}
}

// toUIManipuLayers converts a slice of `types.ManipuLayer` to DTOs,
// best-effort resolving each layer's image URL from its linked upstream
// and manifest.
func toUIManipuLayers(layers []types.ManipuLayer) []UIManipuLayer {
	if len(layers) == 0 {
		return nil
	}
	out := make([]UIManipuLayer, 0, len(layers))
	for _, lyr := range layers {
		entry := UIManipuLayer{
			Rule:   lyr.Rule,
			Select: lyr.Select,
		}
		if up := lyr.Image.Upstream; up.Repo != "" {
			entry.Upstream = up.String()
			if up.Tag != "" {
				entry.Image = up.Reg + "/" + up.Repo + ":" + up.Tag
			}
		}
		out = append(out, entry)
	}
	return out
}
