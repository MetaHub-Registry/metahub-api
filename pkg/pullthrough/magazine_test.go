package pullthrough

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
	mhtypes "github.com/metahub-registry/metahub-core/pkg/types"
)

func testMagazine() magazine.Magazine {
	return magazine.Magazine{Config: magazine.MagConfig{Registries: []magazine.RegistryConfig{
		{
			Name:      "harbor.example.com",
			URL:       "harbor.example.com",
			AllowAnon: true,
			PullThrough: magazine.PullThroughConfig{
				Enabled:        true,
				Cache:          "async",
				ClientRedirect: true,
				MaxCacheBytes:  1024,
				Workers:        7,
			},
		},
		{
			Name:      "off.example",
			AllowAnon: true,
			PullThrough: magazine.PullThroughConfig{
				Enabled: false,
			},
		},
		{
			// Enabled but no anonymous pull → not eligible.
			Name:      "noanon.example",
			AllowAnon: false,
			PullThrough: magazine.PullThroughConfig{
				Enabled: true,
				Cache:   "async",
			},
		},
	}}}
}

func TestMagazinePolicies_PolicyFor(t *testing.T) {
	p := MagazinePolicies{Mag: testMagazine()}

	pol, ok := p.PolicyFor("harbor.example.com")
	require.True(t, ok, "registry must resolve by EffectiveURL/slug")
	assert.True(t, pol.Enabled)
	assert.Equal(t, "async", pol.Cache)
	assert.True(t, pol.ClientRedirect, "client_redirect must be surfaced to the blob router")
	assert.Equal(t, int64(1024), pol.MaxCacheBytes)
	assert.Equal(t, 7, pol.Workers)

	off, ok := p.PolicyFor("off.example")
	require.True(t, ok)
	assert.False(t, off.Enabled, "pull_through.enabled:false must be ineligible")

	noAnon, ok := p.PolicyFor("noanon.example")
	require.True(t, ok)
	assert.False(t, noAnon.Enabled, "allow_anon:false must be ineligible even when enabled")

	_, ok = p.PolicyFor("unknown.example")
	assert.False(t, ok, "unknown registry must not resolve")
}

func TestMagazinePolicies_MaxWorkers(t *testing.T) {
	p := MagazinePolicies{Mag: testMagazine()}
	assert.Equal(t, 7, p.MaxWorkers(), "largest eligible registry override wins")

	empty := MagazinePolicies{Mag: magazine.Magazine{}}
	assert.Equal(t, 0, empty.MaxWorkers())
}

// fakeRefreshDB embeds the magazine.Database interface and reports a missing
// collection, so HeadManifestNotFound fails fast without network/DB.
type fakeRefreshDB struct {
	magazine.Database
}

func (fakeRefreshDB) CollectionGetByRepoTagWoManifests(_ context.Context, _, _ string) (mhtypes.Repo, bool, error) {
	return mhtypes.Repo{}, false, nil
}

// TestMagazineRefresher_CollectionMissing covers the Refresher adapter: a missing
// collection surfaces as an error (the caller logs and keeps serving stale).
func TestMagazineRefresher_CollectionMissing(t *testing.T) {
	r := MagazineRefresher{Mag: magazine.Magazine{Database: fakeRefreshDB{}}}
	err := r.Refresh(context.Background(), RefreshJob{Registry: "reg", Repo: "harbor.example.com/library/alpine", Tag: "3.24"})
	require.Error(t, err)
}
