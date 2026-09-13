// Package pullthrough implements the asynchronous, bounded pull-through cache
// worker: it streams upstream layer blobs into MetaHub storage off the client
// request path, deduplicated per digest and bounded by a per-registry soft cap.
//
// See openspec/changes/add-pull-through-manifest-intercept (upstream-blob-delivery).
package pullthrough

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// CacheModeAsync is the only mode that enqueues cache jobs. "off" (and the
// empty string, which config normalisation should have defaulted) disables.
const CacheModeAsync = "async"

// DefaultWorkers is the in-process pool size when neither the global option nor
// a per-registry override is set.
const DefaultWorkers = 4

// DefaultQueueSize bounds the pending-job channel.
const DefaultQueueSize = 1024

// Job identifies an upstream layer to cache. Registry is the routing key used to
// resolve the per-registry pull-through policy.
type Job struct {
	Registry  string
	Repo      string
	Tag       string
	Digest    string
	MediaType string
}

// Store abstracts MetaHub storage + upstream fetch so the cache worker can be
// unit-tested without a live MinIO/Neo4j/registry.
type Store interface {
	// Exists reports whether the digest's bytes are already in MetaHub storage.
	Exists(ctx context.Context, digest string) (bool, error)
	// Cache streams the upstream layer for job into MetaHub storage and returns
	// the number of bytes written. It MUST stream (never buffer the whole layer).
	Cache(ctx context.Context, job Job) (int64, error)
}

// RegistryPolicy is the per-registry pull-through behaviour the cache honours.
type RegistryPolicy struct {
	Enabled       bool
	Cache         string // "off" | "async"
	MaxCacheBytes int64  // 0 = unlimited
	Workers       int    // 0 = DefaultWorkers
	// ClientRedirect selects the blob delivery mode on a local miss:
	// true → 307 the client to the upstream URL; false → proxy the bytes.
	ClientRedirect bool
	// RefreshInterval is the cached-manifest TTL; 0 means always re-check.
	RefreshInterval time.Duration
}

// PolicyResolver resolves the pull-through policy for a registry key. ok is
// false when the key is not a configured pull-through registry.
type PolicyResolver interface {
	PolicyFor(registry string) (RegistryPolicy, bool)
}

// Metrics is the observability sink. Implementations MUST be safe for
// concurrent use; a nil sink is tolerated.
type Metrics interface {
	Enqueued()
	Dropped(reason string)
	Started()
	Completed(bytes int64)
	Failed()
	QueueDepth(n int)
}

// Options configures the pool. Zero values fall back to the package defaults.
type Options struct {
	Workers   int
	QueueSize int
}

// Cache is a bounded, in-process worker pool. The zero value is not usable;
// construct with New and drive with Start/Stop.
type Cache struct {
	store     Store
	policies  PolicyResolver
	metrics   Metrics
	refresher Refresher
	opts      Options

	jobs     chan Job
	refreshes chan RefreshJob

	mu              sync.Mutex
	inflight        map[string]struct{} // dedup: queued or running digests
	refreshInflight map[string]struct{} // dedup: queued or running (repo:tag)
	cachedBytes     map[string]int64    // per-registry running total

	startOnce sync.Once
	stopOnce  sync.Once
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// New constructs a Cache. store and policies are required; metrics may be nil.
func New(store Store, policies PolicyResolver, metrics Metrics, opts Options) *Cache {
	if opts.Workers <= 0 {
		opts.Workers = DefaultWorkers
	}
	if opts.QueueSize <= 0 {
		opts.QueueSize = DefaultQueueSize
	}
	if metrics == nil {
		metrics = nopMetrics{}
	}
	return &Cache{
		store:           store,
		policies:        policies,
		metrics:         metrics,
		opts:            opts,
		jobs:            make(chan Job, opts.QueueSize),
		refreshes:       make(chan RefreshJob, opts.QueueSize),
		inflight:        map[string]struct{}{},
		refreshInflight: map[string]struct{}{},
		cachedBytes:     map[string]int64{},
	}
}

// Start launches the worker pool. It is idempotent; the pool drains when ctx is
// cancelled or Stop is called.
func (c *Cache) Start(ctx context.Context) {
	c.startOnce.Do(func() {
		runCtx, cancel := context.WithCancel(ctx)
		c.cancel = cancel
		workers := c.opts.Workers
		for i := 0; i < workers; i++ {
			c.wg.Add(1)
			go c.worker(runCtx)
		}
		if c.refresher != nil {
			for i := 0; i < refreshWorkers; i++ {
				c.wg.Add(1)
				go c.refreshWorker(runCtx)
			}
		}
		logrus.WithField("pull_through", true).Infof("pull-through cache started (workers=%d queue=%d refresh=%t)", workers, c.opts.QueueSize, c.refresher != nil)
	})
}

// Stop cancels the pool and waits for in-flight jobs to finish.
func (c *Cache) Stop() {
	c.stopOnce.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
		c.wg.Wait()
	})
}

// Policy resolves the pull-through policy for a registry key (nil-safe). The
// blob handler consults it to pick the delivery mode on a local miss.
func (c *Cache) Policy(registry string) (RegistryPolicy, bool) {
	if c == nil {
		return RegistryPolicy{}, false
	}
	return c.policies.PolicyFor(registry)
}

// Enqueue submits job for asynchronous caching. It never blocks: returns false
// when the job is not eligible (unknown/disabled registry, cache off), already
// queued/running, over the registry's soft cap, or the queue is full.
func (c *Cache) Enqueue(job Job) bool {
	pol, ok := c.policies.PolicyFor(job.Registry)
	if !ok || !pol.Enabled || pol.Cache != CacheModeAsync {
		return false
	}
	if pol.MaxCacheBytes > 0 && c.bytesFor(job.Registry) >= pol.MaxCacheBytes {
		c.metrics.Dropped("max_cache_bytes")
		return false
	}
	if !c.claim(job.Digest) {
		// Already queued or running — deduplicate silently.
		return false
	}
	select {
	case c.jobs <- job:
		c.metrics.Enqueued()
		c.metrics.QueueDepth(len(c.jobs))
		return true
	default:
		c.release(job.Digest)
		c.metrics.Dropped("queue_full")
		logrus.WithField("pull_through", true).Warnf("pull-through cache queue full, dropping digest %s", job.Digest)
		return false
	}
}

// claim reserves digest in the inflight set, returning false if already present.
func (c *Cache) claim(digest string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.inflight[digest]; exists {
		return false
	}
	c.inflight[digest] = struct{}{}
	return true
}

// release removes digest from the inflight set.
func (c *Cache) release(digest string) {
	c.mu.Lock()
	delete(c.inflight, digest)
	c.mu.Unlock()
}

// bytesFor returns the cached-bytes total for a registry key.
func (c *Cache) bytesFor(registry string) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cachedBytes[registry]
}

// addBytes records bytes cached for a registry key.
func (c *Cache) addBytes(registry string, n int64) {
	c.mu.Lock()
	c.cachedBytes[registry] += n
	c.mu.Unlock()
}

// worker consumes jobs until the context is cancelled.
func (c *Cache) worker(ctx context.Context) {
	defer c.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-c.jobs:
			c.metrics.QueueDepth(len(c.jobs))
			c.run(ctx, job)
		}
	}
}

// run executes a single cache job, always releasing the dedup claim.
func (c *Cache) run(ctx context.Context, job Job) {
	defer c.release(job.Digest)
	c.metrics.Started()
	if exists, err := c.store.Exists(ctx, job.Digest); err == nil && exists {
		logrus.WithField("pull_through", true).Debugf("cache skip (already stored): %s", job.Digest)
		return
	} else if err != nil {
		logrus.WithField("pull_through", true).Warnf("cache stat failed for %s: %v", job.Digest, err)
	}
	n, err := c.store.Cache(ctx, job)
	if err != nil {
		c.metrics.Failed()
		logrus.WithField("pull_through", true).Warnf("cache job failed for %s: %v", job.Digest, err)
		return
	}
	c.addBytes(job.Registry, n)
	c.metrics.Completed(n)
}

// nopMetrics is the default Metrics sink.
type nopMetrics struct{}

func (nopMetrics) Enqueued()            {}
func (nopMetrics) Dropped(string)       {}
func (nopMetrics) Started()             {}
func (nopMetrics) Completed(int64)      {}
func (nopMetrics) Failed()              {}
func (nopMetrics) QueueDepth(int)       {}
