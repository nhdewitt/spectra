package server

import (
	"net/http"
	"sync"
	"time"
)

// rateLimiter implements a per-IP token bucket rate limiter.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64       // tokens per second
	burst   int           // max tokens
	cleanup time.Duration // how often to remove stale entries
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
	}

	go rl.cleanupLoop()
	return rl
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

	for range ticker.C {
		rl.mu.Lock()
		stale := time.Now().Add(-rl.cleanup)
		for key, b := range rl.buckets {
			if b.lastSeen.Before(stale) {
				delete(rl.buckets, key)
			}
		}
		rl.mu.Unlock()
	}
}

func (s *Server) rateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !s.Limiter.allow(ip) {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}
