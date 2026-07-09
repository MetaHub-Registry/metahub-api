package handlers

import (
	"context"
	"testing"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

// TestResolveModelCentricRuntime_Guards covers the guard branches that return
// before any DB access (so a zero Handlers is safe): non-model repos and the
// internal -layer/-config repackage outputs are NOT model-centric pulls.
func TestResolveModelCentricRuntime_Guards(t *testing.T) {
	h0 := Handlers{}
	cases := []string{
		"apps/llama-server",           // not a models/ repo
		"qnib/ftest",                  // not a models/ repo
		"models/qnib/minicpm5-layer",  // internal repackage output
		"models/qnib/minicpm5-config", // internal repackage output
	}
	for _, repo := range cases {
		if _, _, _, ok := h0.resolveModelCentricRuntime(context.Background(), types.Request{Repo: repo, Tag: "q8-0"}); ok {
			t.Fatalf("repo %q must not be treated as a model-centric pull", repo)
		}
	}
}
