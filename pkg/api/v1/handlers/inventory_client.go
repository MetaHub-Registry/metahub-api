package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/metahub-registry/metahub-api/pkg/config"
)

// InventoryClient is metahub-api's thin consumer of the inventory service
// (metahub-admin-ui). It ships NO resolver logic: it asks one question over
// one versioned surface — GET /api/v1/resolve?ip=<ip> — and fail-opens:
// every non-200, timeout, or malformed answer becomes ("", false), which the
// AuthHandler treats as "keep generic".
//
// Results are cached per client IP — positive AND negative — for the
// configured cache_ttl (lazy expiry on read, no background goroutine), so
// repeated auths from one node don't hammer the inventory and an inventory
// outage degrades to generic without a request storm.
type InventoryClient struct {
	url    string // base URL, no trailing slash
	token  string // bearer token from token_env (optional)
	http   *http.Client
	ttl    time.Duration
	now    func() time.Time // test seam
	mu     sync.RWMutex
	cache  map[string]inventoryCacheEntry
	warned bool // token_env-unset warning fires once
}

type inventoryCacheEntry struct {
	profile string // empty ⇒ negative outcome
	ok      bool
	expires time.Time
}

// NewInventoryClient builds a client from the validated config. The bearer
// token comes from cfg.TokenEnv (environment, never YAML); an unset variable
// means "no token" and warns once.
func NewInventoryClient(cfg config.InventoryResolutionConfig) *InventoryClient {
	c := &InventoryClient{
		url:   cfg.URL,
		token: cfg.TokenFromEnv(),
		http:  &http.Client{Timeout: cfg.Timeout},
		ttl:   cfg.CacheTTL,
		now:   time.Now,
		cache: make(map[string]inventoryCacheEntry),
	}
	log.WithFields(log.Fields{
		"url":       c.url,
		"timeout":   cfg.Timeout,
		"cache_ttl": cfg.CacheTTL,
		"token":     c.token != "",
	}).Info("profile resolution enabled — inventory service configured")
	return c
}

// resolveResponse is the inventory service's documented v1 shape.
type resolveResponse struct {
	IP       string `json:"ip"`
	Profile  string `json:"profile"`
	Hostname string `json:"hostname,omitempty"`
	Source   string `json:"source"`
}

// Resolve returns the profile for ip, or ok=false when there is no match
// (or anything went wrong — consumers fail open to generic). A nil receiver
// is the disabled state and short-circuits to no-match.
func (c *InventoryClient) Resolve(ctx context.Context, ip string) (string, bool) {
	if c == nil || c.url == "" || ip == "" {
		return "", false
	}

	// Cache check (incl. negative outcomes) — lazy expiry on read.
	c.mu.RLock()
	entry, hit := c.cache[ip]
	c.mu.RUnlock()
	if hit && c.now().Before(entry.expires) {
		log.WithFields(log.Fields{
			"ip": ip, "profile": entry.profile, "ok": entry.ok, "cache": "hit",
		}).Debug("inventory resolve (cached)")
		return entry.profile, entry.ok
	}

	profile, ok := c.resolveHTTP(ctx, ip)

	c.mu.Lock()
	c.cache[ip] = inventoryCacheEntry{
		profile: profile,
		ok:      ok,
		expires: c.now().Add(c.ttl),
	}
	c.mu.Unlock()

	log.WithFields(log.Fields{
		"ip": ip, "profile": profile, "ok": ok, "cache": "miss",
	}).Debug("inventory resolve")
	return profile, ok
}

// resolveHTTP performs the versioned resolve call. ANY failure — timeout,
// connection error, non-200, malformed body, empty profile — becomes
// ("", false): the resolve path can never fail a login.
func (c *InventoryClient) resolveHTTP(ctx context.Context, ip string) (string, bool) {
	endpoint := fmt.Sprintf("%s/api/v1/resolve?ip=%s", c.url, url.QueryEscape(ip))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		log.WithError(err).WithField("ip", ip).Debug("inventory resolve: bad request")
		return "", false
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		log.WithError(err).WithField("ip", ip).Debug("inventory resolve: request failed")
		return "", false
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var body resolveResponse
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.Profile == "" {
			log.WithFields(log.Fields{"ip": ip}).Debug("inventory resolve: malformed/empty 200 body")
			return "", false
		}
		return body.Profile, true
	case http.StatusNotFound:
		// Documented no-match — the normal negative outcome.
		return "", false
	default:
		log.WithFields(log.Fields{"ip": ip, "status": resp.StatusCode}).
			Debug("inventory resolve: unexpected status")
		return "", false
	}
}
