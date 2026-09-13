package pullthrough

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

// refreshWorkers bounds the manifest-refresh pool, kept separate from the layer
// download pool so a slow refresh cannot starve caching.
const refreshWorkers = 2

// RefreshJob identifies a cached pull-through (repo, tag) to re-check upstream.
// Registry is the upstream registry key (EffectiveURL).
type RefreshJob struct {
	Registry string
	Repo     string
	Tag      string
	Profile  string
}

// Refresher re-discovers an upstream (repo, tag) and rewires the Slot when the
// upstream digest moved. Implemented by MagazineRefresher.
type Refresher interface {
	Refresh(ctx context.Context, job RefreshJob) error
}

// NeedsRefresh reports whether a cached pull-through (repo, tag) is due for an
// upstream re-check. A missing/zero last_refreshed and a non-positive interval
// always refresh; digest pulls bypass this at the call site.
func NeedsRefresh(lastRefreshed int64, found bool, now time.Time, interval time.Duration) bool {
	if !found || lastRefreshed <= 0 {
		return true
	}
	if interval <= 0 {
		return true
	}
	return now.Unix()-lastRefreshed >= int64(interval.Seconds())
}

// WithRefresher attaches a Refresher. Call before Start (builder).
func (c *Cache) WithRefresher(r Refresher) *Cache {
	c.refresher = r
	return c
}

// EnqueueRefresh schedules an async upstream re-check for (repo, tag). It never
// blocks and deduplicates per (repo, tag); returns false when disabled or a
// refresh for the pair is already queued/running.
func (c *Cache) EnqueueRefresh(job RefreshJob) bool {
	if c == nil || c.refresher == nil {
		return false
	}
	key := job.Repo + ":" + job.Tag
	if !c.claimRefresh(key) {
		return false
	}
	select {
	case c.refreshes <- job:
		return true
	default:
		c.releaseRefresh(key)
		logrus.WithField("pull_through", true).Warnf("pull-through refresh queue full, dropping %s", key)
		return false
	}
}

func (c *Cache) claimRefresh(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.refreshInflight[key]; exists {
		return false
	}
	c.refreshInflight[key] = struct{}{}
	return true
}

func (c *Cache) releaseRefresh(key string) {
	c.mu.Lock()
	delete(c.refreshInflight, key)
	c.mu.Unlock()
}

// refreshWorker consumes refresh jobs until the context is cancelled. It always
// releases the dedup claim, and on error logs (the client was already served
// stale by the intercept).
func (c *Cache) refreshWorker(ctx context.Context) {
	defer c.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-c.refreshes:
			if err := c.refresher.Refresh(ctx, job); err != nil {
				logrus.WithField("pull_through", true).Warnf("manifest refresh failed for %s:%s: %v", job.Repo, job.Tag, err)
			} else {
				logrus.WithField("pull_through", true).Debugf("manifest refresh done for %s:%s", job.Repo, job.Tag)
			}
			c.releaseRefresh(job.Repo + ":" + job.Tag)
		}
	}
}
