package pullthrough

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNeedsRefresh(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	hour := time.Hour

	assert.False(t, NeedsRefresh(now.Add(-10*time.Minute).Unix(), true, now, hour),
		"within TTL must not refresh")
	assert.True(t, NeedsRefresh(now.Add(-2*time.Hour).Unix(), true, now, hour),
		"past TTL must refresh")
	assert.True(t, NeedsRefresh(0, true, now, hour), "zero last_refreshed must refresh")
	assert.True(t, NeedsRefresh(0, false, now, hour), "missing last_refreshed must refresh")
	assert.True(t, NeedsRefresh(now.Unix(), true, now, 0), "interval 0 must always refresh")
	assert.True(t, NeedsRefresh(now.Unix(), true, now, -time.Second), "negative interval must refresh")
}

// fakeRefresher records Refresh calls and can inject an error.
type fakeRefresher struct {
	mu    sync.Mutex
	calls []string
	err   error
}

func (f *fakeRefresher) Refresh(_ context.Context, job RefreshJob) error {
	f.mu.Lock()
	f.calls = append(f.calls, job.Repo+":"+job.Tag)
	f.mu.Unlock()
	return f.err
}

func (f *fakeRefresher) called() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func newRefreshCache(t *testing.T, r Refresher) (*Cache, *recordingMetrics) {
	t.Helper()
	m := newRecordingMetrics()
	c := New(newFakeStore(), mapPolicies{"reg": enabledPolicy()}, m, Options{Workers: 1, QueueSize: 4}).
		WithRefresher(r)
	ctx, cancel := context.WithCancel(context.Background())
	c.Start(ctx)
	t.Cleanup(func() { cancel(); c.Stop() })
	return c, m
}

func refreshJob() RefreshJob {
	return RefreshJob{Registry: "reg", Repo: "harbor.example.com/library/alpine", Tag: "3.24", Profile: "generic"}
}

// TestEnqueueRefresh_Dedup: only one refresh runs per (repo, tag) while queued.
func TestEnqueueRefresh_Dedup(t *testing.T) {
	r := &fakeRefresher{}
	c, _ := newRefreshCache(t, r)

	require.True(t, c.EnqueueRefresh(refreshJob()), "first refresh accepted")
	assert.False(t, c.EnqueueRefresh(refreshJob()), "duplicate while queued/running must dedup")

	require.Eventually(t, func() bool { return len(r.called()) == 1 }, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, []string{"harbor.example.com/library/alpine:3.24"}, r.called())
}

// TestEnqueueRefresh_NoRefresher: without a Refresher the cache never enqueues.
func TestEnqueueRefresh_NoRefresher(t *testing.T) {
	c := New(newFakeStore(), mapPolicies{"reg": enabledPolicy()}, newRecordingMetrics(), Options{})
	assert.False(t, c.EnqueueRefresh(refreshJob()))
}

// TestRefresh_UpstreamDown_ReleasesClaim: a failing refresh (upstream down) logs
// and releases the dedup claim so a later access can retry.
func TestRefresh_UpstreamDown_ReleasesClaim(t *testing.T) {
	r := &fakeRefresher{err: errors.New("upstream unreachable")}
	c, _ := newRefreshCache(t, r)

	require.True(t, c.EnqueueRefresh(refreshJob()))
	require.Eventually(t, func() bool { return len(r.called()) == 1 }, 2*time.Second, 10*time.Millisecond)
	// Claim released after the failed run → the next access can re-enqueue.
	// The refresher records the call before the worker sees the error and
	// releases the claim, so poll instead of asserting immediately.
	require.Eventually(t, func() bool { return c.EnqueueRefresh(refreshJob()) },
		2*time.Second, 10*time.Millisecond, "claim must be released after a failed refresh")
}
