package handlers

import (
	"context"
	"testing"

	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func TestProfileForIndex(t *testing.T) {
	modelCtx := types.WithRequestedModel(context.Background(), "e2e-gguf37")
	cases := []struct {
		name         string
		ctx          context.Context
		req          types.Request
		modelServing bool
		want         string
	}{
		{
			name: "empty profile is generic",
			ctx:  context.Background(),
			req:  types.Request{Repo: "qnib/ftest"},
			want: genericProfile,
		},
		{
			name:         "collection under a model-serving profile is generic",
			ctx:          context.Background(),
			req:          types.Request{Repo: "qnib/ftest", Profile: "e2e-inject"},
			modelServing: true,
			want:         genericProfile,
		},
		{
			name:         "collection under an ordinary profile keeps it",
			ctx:          context.Background(),
			req:          types.Request{Repo: "qnib/ftest", Profile: "glow#1.5.0"},
			modelServing: false,
			want:         "glow#1.5.0",
		},
		{
			name:         "model repo keeps the profile",
			ctx:          context.Background(),
			req:          types.Request{Repo: "models/e2e-gguf37", Profile: "e2e-inject-37"},
			modelServing: true,
			want:         "e2e-inject-37",
		},
		{
			name:         "app repo keeps the profile",
			ctx:          context.Background(),
			req:          types.Request{Repo: "apps/testmodel-runtime", Profile: "bazzite"},
			modelServing: true,
			want:         "bazzite",
		},
		{
			name:         "runtime repo after the model-centric rewrite keeps the profile",
			ctx:          modelCtx,
			req:          types.Request{Repo: "qnib/ftest", Profile: "e2e-inject-37"},
			modelServing: true,
			want:         "e2e-inject-37",
		},
		{
			name:         "generic stays generic with the model context",
			ctx:          modelCtx,
			req:          types.Request{Repo: "qnib/ftest", Profile: genericProfile},
			modelServing: true,
			want:         genericProfile,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := profileForIndex(tc.ctx, tc.req, tc.modelServing); got != tc.want {
				t.Fatalf("profileForIndex() = %q, want %q", got, tc.want)
			}
		})
	}
}

// fakeProfileDB embeds the magazine.Database interface so tests only have to
// implement the one lookup the profile-gate helpers use.
type fakeProfileDB struct {
	magazine.Database
	profile types.Profile
	found   bool
	err     error
}

func (f fakeProfileDB) ProfileGetByLabel(context.Context, string) (types.Profile, bool, error) {
	return f.profile, f.found, f.err
}

func modelServingProfile() types.Profile {
	return types.Profile{
		Label: "bazzite",
		Hardware: &types.Hardware{
			GPU: &types.HardwareGPU{Vendor: "nvidia", VRAMGB: 24},
		},
	}
}

func TestProfileIsModelServing(t *testing.T) {
	cases := []struct {
		name  string
		db    fakeProfileDB
		label string
		want  bool
	}{
		{name: "model-serving profile", db: fakeProfileDB{profile: modelServingProfile(), found: true}, label: "bazzite", want: true},
		{name: "ordinary profile", db: fakeProfileDB{profile: types.Profile{Label: "glow#1.5.0"}, found: true}, label: "glow#1.5.0", want: false},
		{name: "not found", db: fakeProfileDB{}, label: "missing", want: false},
		{name: "lookup error", db: fakeProfileDB{err: context.Canceled}, label: "bazzite", want: false},
		{name: "empty label", db: fakeProfileDB{profile: modelServingProfile(), found: true}, label: "", want: false},
		{name: "generic label", db: fakeProfileDB{profile: modelServingProfile(), found: true}, label: genericProfile, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := profileIsModelServing(context.Background(), tc.db, tc.label); got != tc.want {
				t.Fatalf("profileIsModelServing() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStoredProfileOrGeneric(t *testing.T) {
	cases := []struct {
		name  string
		db    fakeProfileDB
		label string
		want  string
	}{
		{name: "stored profile", db: fakeProfileDB{profile: modelServingProfile(), found: true}, label: "bazzite", want: "bazzite"},
		{name: "unknown profile", db: fakeProfileDB{}, label: "e2e/26", want: genericProfile},
		{name: "lookup error", db: fakeProfileDB{err: context.Canceled}, label: "bazzite", want: genericProfile},
		{name: "empty label", db: fakeProfileDB{}, label: "", want: ""},
		{name: "generic label", db: fakeProfileDB{}, label: genericProfile, want: genericProfile},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := storedProfileOrGeneric(context.Background(), tc.db, tc.label); got != tc.want {
				t.Fatalf("storedProfileOrGeneric() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestShouldApplySingleIndex(t *testing.T) {
	modelCtx := types.WithRequestedModel(context.Background(), "e2e-gguf37")
	cases := []struct {
		name string
		ctx  context.Context
		req  types.Request
		prf  types.Profile
		want bool
	}{
		{name: "no profile", ctx: context.Background(), req: types.Request{Repo: "qnib/ftest"}, prf: modelServingProfile(), want: false},
		{name: "generic profile", ctx: context.Background(), req: types.Request{Repo: "qnib/ftest", Profile: genericProfile}, prf: modelServingProfile(), want: false},
		{name: "model-serving on a collection", ctx: context.Background(), req: types.Request{Repo: "qnib/ftest", Profile: "bazzite"}, prf: modelServingProfile(), want: false},
		{name: "model-serving on a model repo", ctx: context.Background(), req: types.Request{Repo: "models/x", Profile: "bazzite"}, prf: modelServingProfile(), want: true},
		{name: "model-serving runtime with requested model", ctx: modelCtx, req: types.Request{Repo: "qnib/ftest", Profile: "bazzite"}, prf: modelServingProfile(), want: true},
		{name: "ordinary profile on a collection", ctx: context.Background(), req: types.Request{Repo: "qnib/ftest", Profile: "glow#1.5.0"}, prf: types.Profile{Label: "glow#1.5.0"}, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldApplySingleIndex(tc.ctx, tc.req, tc.prf); got != tc.want {
				t.Fatalf("shouldApplySingleIndex() = %v, want %v", got, tc.want)
			}
		})
	}
}
