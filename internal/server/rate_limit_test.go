package server

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestRateLimiter_AllowsWithinBurst(t *testing.T) {
	rl := newRateLimiter(10, 5) // 10/sec, burst of 5

	for i := range 5 {
		if !rl.allow("192.168.1.1") {
			t.Errorf("request %d should be allowed within burst", i+1)
		}
	}
}

func TestRateLimiter_BlocksOverBurst(t *testing.T) {
	rl := newRateLimiter(10, 3)

	for range 3 {
		rl.allow("192.168.1.1")
	}

	if rl.allow("192.168.1.1") {
		t.Error("request should be blocked after burst exhausted")
	}
}

func TestRateLimiter_SeparateIPs(t *testing.T) {
	rl := newRateLimiter(10, 2)

	// Exhaust IP 1
	rl.allow("192.168.1.1")
	rl.allow("192.168.1.1")

	// IP 2 should still work
	if !rl.allow("192.168.1.2") {
		t.Error("different IP should have its own bucket")
	}
}

func TestRateLimiter_Hammer(t *testing.T) {
	rl := newRateLimiter(10, 30)

	var allowed, blocked int

	// Fire 1000 requests as fast as possible
	for range 1000 {
		if rl.allow("192.168.1.1") {
			allowed++
		} else {
			blocked++
		}
	}

	// Burst of 30 plus whatever comes in from refill during the loop (~1ms)
	if allowed > 40 {
		t.Errorf("allowed %d, expected ~30-35", allowed)
	}
	if blocked == 0 {
		t.Error("should have blocked some requests")
	}

	t.Logf("allowed: %d, blocked: %d", allowed, blocked)
}

func TestRateLimiter_Concurrent(t *testing.T) {
	rl := newRateLimiter(100, 50)
	var wg sync.WaitGroup

	allowed := make(chan bool, 200)

	for range 200 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed <- rl.allow("192.168.1.1")
		}()
	}

	wg.Wait()
	close(allowed)

	trueCount := 0
	for a := range allowed {
		if a {
			trueCount++
		}
	}

	// Should allow exactly burst amount
	if trueCount > 50 {
		t.Errorf("allowed %d, expected at most 50", trueCount)
	}
	if trueCount < 1 {
		t.Error("should allow at least 1 request")
	}
}

func TestClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"

	got := clientIP(req)
	if got != "10.0.0.1" {
		t.Errorf("clientIP = %s, want 10.0.0.1", got)
	}
}

func TestRateLimit_Middleware(t *testing.T) {
	s, _, _, _ := newTestServer()
	s.Limiter = newRateLimiter(100, 2)

	handler := s.rateLimit(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// First 2 should pass
	for i := range 2 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("request %d: got %d, want 200", i+1, rec.Code)
		}
	}

	// 3rd should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("request 3: got %d, want 429", rec.Code)
	}
}

func BenchmarkRateLimiter_Allow(b *testing.B) {
	rl := newRateLimiter(1000, 100)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		rl.allow("192.168.1.1")
	}
}
