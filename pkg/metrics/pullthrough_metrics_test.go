package metrics

import "testing"

// TestPullThroughMetrics exercises the pull-through counters/gauges and the
// PullThroughMetrics sink adapter (both the enabled and disabled paths).
func TestPullThroughMetrics(t *testing.T) {
	SetEnabled(true)
	defer SetEnabled(true)

	PullThroughCacheEvent("enqueued", "")
	PullThroughCacheEvent("dropped", "queue_full")
	PullThroughBytesCached(10)
	PullThroughBlobOutcome("redirect")
	PullThroughBlobOutcome("proxy")
	PullThroughBlobOutcome("hit")
	PullThroughRefreshOutcome("enqueued")
	PullThroughRefreshOutcome("ok")
	PullThroughRefreshOutcome("failed")
	PullThroughQueueDepth(3)

	var m PullThroughMetrics
	m.Enqueued()
	m.Dropped("max_cache_bytes")
	m.Started()
	m.Completed(5)
	m.Failed()
	m.QueueDepth(1)
	m.RefreshOutcome("ok")

	// Disabled path: instrumentation is a no-op but must not panic.
	SetEnabled(false)
	PullThroughCacheEvent("enqueued", "")
	PullThroughBytesCached(1)
	PullThroughBlobOutcome("hit")
	PullThroughRefreshOutcome("ok")
	PullThroughQueueDepth(0)
	m.Enqueued()
}
