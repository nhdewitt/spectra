package server

import (
	"net/http"
	"sync"
	"time"
)

// Rate limit tiers.
// Authenticated dashboard users get a higher limit since pages
// legitimately fan out to many API endpoints on load.
const (
	anonRate  = 10.0
	anonBurst = 30

	authedRate  = 50.0
	authedBurst = 100

	agentRate  = 10.0
	agentBurst = 30
)

// rateLimiter implements a per-key token bucket rate limiter.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64       // tokens per second
	burst   int           // max tokens
	cleanup time.Duration // how often to remove stale entries
	done    chan struct{}
}

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

func newRateLimiter(rate float64, burst int) *rateLimiter {
	rl := &rateLimiter{
		buckets: make(map[string]*bucket),
		rate:    rate,
		burst:   burst,
		cleanup: 5 * time.Minute,
		done:    make(chan struct{}),
	}

	go rl.cleanupLoop()
	return rl
}

func (rl *rateLimiter) Stop() {
	close(rl.done)
}

// allow checks whether the given IP has tokens available.
func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, exists := rl.buckets[key]
	if !exists {
		rl.buckets[key] = &bucket{
			tokens:   float64(rl.burst) - 1,
			lastSeen: now,
		}
		return true
	}

	// Refill based on elapsed time
	elapsed := now.Sub(b.lastSeen).Seconds()
	b.tokens = min(b.tokens+elapsed*rl.rate, float64(rl.burst))
	b.lastSeen = now

	if b.tokens < 1 {
		return false
	}

	b.tokens--
	return true
}

// cleanupLoop periodically removes stale bucket entries.
func (rl *rateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			stale := time.Now().Add(-rl.cleanup)
			for key, b := range rl.buckets {
				if b.lastSeen.Before(stale) {
					delete(rl.buckets, key)
				}
			}
			rl.mu.Unlock()
		case <-rl.done:
			return
		}
	}
}

// tieredLimiters holds separate rate limiters per caller type.
type tieredLimiters struct {
	anon   *rateLimiter
	authed *rateLimiter
	agent  *rateLimiter
}

func newTieredLimiters() *tieredLimiters {
	return &tieredLimiters{
		anon:   newRateLimiter(anonRate, anonBurst),
		authed: newRateLimiter(authedRate, authedBurst),
		agent:  newRateLimiter(agentRate, agentBurst),
	}
}

func (tl *tieredLimiters) Stop() {
	tl.anon.Stop()
	tl.authed.Stop()
	tl.agent.Stop()
}

// rateLimit applies the anonymous tier (login, register).
func (s *Server) rateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !s.Limiters.anon.allow(ip) {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

// rateLimitAuthed applies the authenticated user tier.
// Keys on username (not IP) so multiple tabs from the same user share a bucket.
// Must be called after requireUserAuth in the middleware chain.
func (s *Server) rateLimitAuthed(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := clientIP(r)
		if u, ok := userFromContext(r.Context()); ok {
			key = "user:" + u.Username
		}
		if !s.Limiters.authed.allow(key) {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

// rateLimitAgent applies the agent tier, keyed by IP.
func (s *Server) rateLimitAgent(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := clientIP(r)
		if agentID := r.Header.Get("X-Agent-ID"); agentID != "" {
			key = "agent:" + agentID
		}
		if !s.Limiters.agent.allow(key) {
			w.Header().Set("Retry-After", "5")
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}
