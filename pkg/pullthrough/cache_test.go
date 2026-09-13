package pullthrough

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingMetrics captures the counters the cache emits.
type recordingMetrics struct {
	mu        sync.Mutex
	enqueued  int
	dropped   map[string]int
	started   int
	completed int
	failed    int
	bytes     int64
	refresh   map[string]int
}

func newRecordingMetrics() *recordingMetrics {
	return &recordingMetrics{dropped: map[string]int{}, refresh: map[string]int{}}
}

func (m *recordingMetrics) Enqueued() {
	m.mu.Lock()
	m.enqueued++
	m.mu.Unlock()
}
func (m *recordingMetrics) Dropped(reason string) {
	m.mu.Lock()
	m.dropped[reason]++
	m.mu.Unlock()
}
func (m *recordingMetrics) Started() {
	m.mu.Lock()
	m.started++
	m.mu.Unlock()
}
func (m *recordingMetrics) Completed(n int64) {
	m.mu.Lock()
	m.completed++
	m.bytes += n
	m.mu.Unlock()
}
func (m *recordingMetrics) Failed() {
	m.mu.Lock()
	m.failed++
	m.mu.Unlock()
}
func (m *recordingMetrics) QueueDepth(int) {}

func (m *recordingMetrics) RefreshOutcome(outcome string) {
	m.mu.Lock()
	m.refresh[outcome]++
	m.mu.Unlock()
}

func (m *recordingMetrics) bytesCached() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.bytes
}

func (m *recordingMetrics) snapshot() (enqueued, started, completed, failed int, dropped map[string]int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d := map[string]int{}
	for k, v := range m.dropped {
		d[k] = v
	}
	return m.enqueued, m.started, m.completed, m.failed, d
}

// fakeStore is a controllable Store.
type fakeStore struct {
	exists   map[string]bool
	size     int64
	block    chan struct{}
	mu       sync.Mutex
	cached   []string
	cacheErr error
}

func newFakeStore() *fakeStore {
	return &fakeStore{exists: map[string]bool{}, size: 42}
}

func (f *fakeStore) Exists(_ context.Context, digest string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.exists[digest], nil
}

func (f *fakeStore) Cache(_ context.Context, job Job) (int64, error) {
	if f.block != nil {
		<-f.block
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cacheErr != nil {
		return 0, f.cacheErr
	}
	f.cached = append(f.cached, job.Digest)
	return f.size, nil
}

func (f *fakeStore) cachedDigests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.cached...)
}

// mapPolicies resolves policies from a fixed map.
type mapPolicies map[string]RegistryPolicy

func (m mapPolicies) PolicyFor(registry string) (RegistryPolicy, bool) {
	p, ok := m[registry]
	return p, ok
}

func enabledPolicy() RegistryPolicy {
	return RegistryPolicy{Enabled: true, Cache: CacheModeAsync}
}

func newTestCache(t *testing.T, store Store, policies PolicyResolver, m *recordingMetrics) (*Cache, context.CancelFunc) {
	t.Helper()
	c := New(store, policies, m, Options{Workers: 1, QueueSize: 8})
	ctx, cancel := context.WithCancel(context.Background())
	c.Start(ctx)
	t.Cleanup(func() {
		cancel()
		c.Stop()
	})
	return c, cancel
}

func job(digest string) Job {
	return Job{Registry: "reg", Repo: "library/alpine", Tag: "3.24", Digest: digest}
}

// TestEnqueue_DisabledRegistryOrCacheOff covers task 1.6: unknown registry and
// cache:off never enqueue.
func TestEnqueue_DisabledRegistryOrCacheOff(t *testing.T) {
	store := newFakeStore()
	m := newRecordingMetrics()
	policies := mapPolicies{
		"off": {Enabled: true, Cache: "off"},
		"on":  enabledPolicy(),
	}
	c, _ := newTestCache(t, store, policies, m)

	assert.False(t, c.Enqueue(job("sha256:unknown")), "unknown registry must not enqueue")
	assert.False(t, c.Enqueue(Job{Registry: "off", Digest: "sha256:off"}), "cache:off must not enqueue")
	assert.True(t, c.Enqueue(Job{Registry: "on", Digest: "sha256:ok"}), "enabled registry must enqueue")

	require.Eventually(t, func() bool {
		_, _, completed, _, _ := m.snapshot()
		return completed == 1
	}, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, []string{"sha256:ok"}, store.cachedDigests())
}

// TestDedup_ConcurrentSameDigest covers task 1.2/1.7: only one job per digest
// while queued or running.
func TestDedup_ConcurrentSameDigest(t *testing.T) {
	store := newFakeStore()
	store.block = make(chan struct{})
	m := newRecordingMetrics()
	c, _ := newTestCache(t, store, mapPolicies{"reg": enabledPolicy()}, m)

	require.True(t, c.Enqueue(job("sha256:dup")), "first enqueue accepted")
	// Wait for the worker to pick it up and block in Cache.
	require.Eventually(t, func() bool {
		_, started, _, _, _ := m.snapshot()
		return started == 1
	}, 2*time.Second, 10*time.Millisecond)
	assert.False(t, c.Enqueue(job("sha256:dup")), "duplicate while running must be deduplicated")

	close(store.block)
	require.Eventually(t, func() bool {
		_, _, completed, _, _ := m.snapshot()
		return completed == 1
	}, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, []string{"sha256:dup"}, store.cachedDigests())
}

// TestDropOnQueueOverflow covers task 1.4/1.7: a full queue drops the job,
// increments the drop metric and never blocks.
func TestDropOnQueueOverflow(t *testing.T) {
	store := newFakeStore()
	store.block = make(chan struct{})
	m := newRecordingMetrics()
	c := New(store, mapPolicies{"reg": enabledPolicy()}, m, Options{Workers: 1, QueueSize: 1})
	ctx, cancel := context.WithCancel(context.Background())
	c.Start(ctx)
	defer func() { cancel(); c.Stop() }()

	require.True(t, c.Enqueue(job("sha256:a")))
	require.Eventually(t, func() bool {
		_, started, _, _, _ := m.snapshot()
		return started == 1
	}, 2*time.Second, 10*time.Millisecond)
	require.True(t, c.Enqueue(job("sha256:b")), "second job fits the queue")
	assert.False(t, c.Enqueue(job("sha256:c")), "third job must be dropped (queue full)")

	_, _, _, _, dropped := m.snapshot()
	assert.Equal(t, 1, dropped["queue_full"])

	close(store.block)
}

// TestStatSkip_AlreadyStored covers task 1.3/1.7: an existing object is skipped.
func TestStatSkip_AlreadyStored(t *testing.T) {
	store := newFakeStore()
	store.exists["sha256:present"] = true
	m := newRecordingMetrics()
	c, _ := newTestCache(t, store, mapPolicies{"reg": enabledPolicy()}, m)

	require.True(t, c.Enqueue(job("sha256:present")))
	require.Eventually(t, func() bool {
		_, started, _, _, _ := m.snapshot()
		return started == 1
	}, 2*time.Second, 10*time.Millisecond)
	assert.Empty(t, store.cachedDigests(), "already-stored digest must not be re-downloaded")
}

// TestMaxCacheBytesSoftCap covers task 1.5/1.7: once cached bytes reach the cap,
// new jobs are dropped.
func TestMaxCacheBytesSoftCap(t *testing.T) {
	store := newFakeStore()
	store.size = 100
	pol := enabledPolicy()
	pol.MaxCacheBytes = 100
	m := newRecordingMetrics()
	c, _ := newTestCache(t, store, mapPolicies{"reg": pol}, m)

	require.True(t, c.Enqueue(job("sha256:first")))
	require.Eventually(t, func() bool {
		_, _, completed, _, _ := m.snapshot()
		return completed == 1
	}, 2*time.Second, 10*time.Millisecond)

	assert.False(t, c.Enqueue(job("sha256:second")), "cap reached must stop enqueuing")
	_, _, _, _, dropped := m.snapshot()
	assert.Equal(t, 1, dropped["max_cache_bytes"])
}

// TestCompletedRecordsBytes covers the bytes-cached accounting.
func TestCompletedRecordsBytes(t *testing.T) {
	store := newFakeStore()
	store.size = 7
	m := newRecordingMetrics()
	c, _ := newTestCache(t, store, mapPolicies{"reg": enabledPolicy()}, m)

	require.True(t, c.Enqueue(job("sha256:x")))
	require.Eventually(t, func() bool {
		_, _, completed, _, _ := m.snapshot()
		return completed == 1
	}, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, int64(7), m.bytesCached())
}
