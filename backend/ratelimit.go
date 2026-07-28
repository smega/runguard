// ratelimit.go
//
// Minimal, dependency-free per-key rate limiting for gin. Deliberately avoids
// pulling in golang.org/x/time/rate (or anything else) so it doesn't require
// touching go.mod/go.sum.
//
// This is an in-memory, single-process limiter: fine for a single backend
// instance, but if RunGuard is ever run with multiple replicas behind a load
// balancer, each replica will enforce its own independent limit. At that
// point this should move to a shared store (e.g. Redis).
package main

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type tokenBucket struct {
	tokens   float64
	lastSeen time.Time
}

// keyedRateLimiter is a token-bucket limiter keyed by an arbitrary string
// (client IP, a URL param, etc). Each key gets its own independent bucket.
type keyedRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	rate    float64 // tokens refilled per second
	burst   float64 // max tokens a bucket can hold (= max burst size)
	idleTTL time.Duration
}

func newKeyedRateLimiter(ratePerMinute, burst float64) *keyedRateLimiter {
	rl := &keyedRateLimiter{
		buckets: make(map[string]*tokenBucket),
		rate:    ratePerMinute / 60.0,
		burst:   burst,
		idleTTL: 10 * time.Minute,
	}
	go rl.janitor()
	return rl
}

// allow reports whether a request for the given key is permitted right now,
// consuming one token if so.
func (rl *keyedRateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[key]
	if !ok {
		rl.buckets[key] = &tokenBucket{tokens: rl.burst - 1, lastSeen: now}
		return true
	}

	elapsed := now.Sub(b.lastSeen).Seconds()
	b.tokens += elapsed * rl.rate
	if b.tokens > rl.burst {
		b.tokens = rl.burst
	}
	b.lastSeen = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// janitor periodically evicts buckets that haven't been touched in a while,
// so the map doesn't grow without bound (e.g. one bucket per attacker IP).
func (rl *keyedRateLimiter) janitor() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-rl.idleTTL)
		rl.mu.Lock()
		for k, b := range rl.buckets {
			if b.lastSeen.Before(cutoff) {
				delete(rl.buckets, k)
			}
		}
		rl.mu.Unlock()
	}
}

// rateLimitByIP returns gin middleware limiting requests per client IP.
//
// Note: gin's ClientIP() trusts X-Forwarded-For from any upstream by default.
// If RunGuard runs behind a reverse proxy/CDN, call r.SetTrustedProxies(...)
// with the real proxy IPs so this can't be bypassed by spoofing that header.
func rateLimitByIP(ratePerMinute, burst float64) gin.HandlerFunc {
	rl := newKeyedRateLimiter(ratePerMinute, burst)
	return func(c *gin.Context) {
		if !rl.allow(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "too many requests, please slow down and try again shortly",
			})
			return
		}
		c.Next()
	}
}

// rateLimitByParam returns gin middleware limiting requests per URL param
// value (e.g. the heartbeat job key), independent of source IP. This stops a
// single key from being flooded regardless of how many source IPs are used.
func rateLimitByParam(param string, ratePerMinute, burst float64) gin.HandlerFunc {
	rl := newKeyedRateLimiter(ratePerMinute, burst)
	return func(c *gin.Context) {
		key := c.Param(param)
		if key == "" {
			c.Next()
			return
		}
		if !rl.allow(key) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "too many requests for this key, please slow down",
			})
			return
		}
		c.Next()
	}
}
