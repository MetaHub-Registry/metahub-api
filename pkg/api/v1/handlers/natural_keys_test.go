package handlers

import (
	"encoding/json"
	"testing"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

func assertNoStorageID(t *testing.T, where string, v any) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("%s: marshal: %v", where, err)
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("%s: unmarshal: %v", where, err)
	}
	var walk func(any)
	walk = func(node any) {
		switch n := node.(type) {
		case map[string]any:
			for k, child := range n {
				if k == "id" || (len(k) > 3 && k[len(k)-3:] == "_id") {
					t.Errorf("%s: storage ID field %q in public payload", where, k)
				}
				walk(child)
			}
		case []any:
			for _, child := range n {
				walk(child)
			}
		}
	}
	walk(decoded)
}

// TestPublicDTOsCarryNoStorageIDs is the response-side guard for the
// natural-key contract. It runs in the integration lane (the handlers
// TestMain needs containers) and covers every DTO that is written to a
// public JSON response.
func TestPublicDTOsCarryNoStorageIDs(t *testing.T) {
	assertNoStorageID(t, "Next", Next{Repo: "qnib/ftest", Tag: "latest"})
	assertNoStorageID(t, "ProfileDetailResponse", ProfileDetailResponse{
		Label: "generic#v1",
		Rules: []ProfileRuleResp{{Type: "anno", Op: "eq", Key: "k", Value: "v"}},
		Manipulations: []UIManipulation{{
			Label: "model/demo",
			Default: &UIManipuConfig{
				Cmd: &UIManipuCommand{Op: "replace", Value: []string{"sh"}},
			},
		}},
	})
	assertNoStorageID(t, "types.OrasArtifact", types.OrasArtifact{
		ID:           42,
		Repo:         "qnib/ftest",
		Digest:       "sha256:abc",
		ArtifactType: "application/vnd.cyclonedx",
	})
	assertNoStorageID(t, "types.Manipulation", types.Manipulation{ID: 42, Label: "model/demo"})
}
