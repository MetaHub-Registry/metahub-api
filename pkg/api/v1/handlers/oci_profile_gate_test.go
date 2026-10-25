package handlers

import (
	"context"
	"testing"

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
