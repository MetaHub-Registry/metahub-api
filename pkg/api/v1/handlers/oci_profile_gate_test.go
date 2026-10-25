package handlers

import (
	"context"
	"testing"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

func TestProfileForIndex(t *testing.T) {
	modelCtx := types.WithRequestedModel(context.Background(), "e2e-gguf37")
	cases := []struct {
		name string
		ctx  context.Context
		req  types.Request
		want string
	}{
		{
			name: "empty profile is generic",
			ctx:  context.Background(),
			req:  types.Request{Repo: "qnib/ftest"},
			want: genericProfile,
		},
		{
			name: "collection pull is generic",
			ctx:  context.Background(),
			req:  types.Request{Repo: "qnib/ftest", Profile: "e2e-inject"},
			want: genericProfile,
		},
		{
			name: "model repo keeps the profile",
			ctx:  context.Background(),
			req:  types.Request{Repo: "models/e2e-gguf37", Profile: "e2e-inject-37"},
			want: "e2e-inject-37",
		},
		{
			name: "app repo keeps the profile",
			ctx:  context.Background(),
			req:  types.Request{Repo: "apps/testmodel-runtime", Profile: "bazzite"},
			want: "bazzite",
		},
		{
			name: "runtime repo after the model-centric rewrite keeps the profile",
			ctx:  modelCtx,
			req:  types.Request{Repo: "qnib/ftest", Profile: "e2e-inject-37"},
			want: "e2e-inject-37",
		},
		{
			name: "generic stays generic with the model context",
			ctx:  modelCtx,
			req:  types.Request{Repo: "qnib/ftest", Profile: genericProfile},
			want: genericProfile,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := profileForIndex(tc.ctx, tc.req); got != tc.want {
				t.Fatalf("profileForIndex() = %q, want %q", got, tc.want)
			}
		})
	}
}
