package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync"
	"testing"
)

func contextWithUser(ctx context.Context, u *userContext) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

func TestRateLimiter_AllowsWithinBurst(t *testing.T) {
	rl := newRateLimiter(10, 5)

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

	rl.allow("192.168.1.1")
	rl.allow("192.168.1.1")

	if !rl.allow("192.168.1.2") {
		t.Error("different IP should have its own bucket")
	}
}

func TestRateLimiter_Hammer(t *testing.T) {
	rl := newRateLimiter(10, 30)

	var allowed, blocked int

	for range 1000 {
		if rl.allow("192.168.1.1") {
			allowed++
		} else {
			blocked++
		}
	}

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

	if trueCount > 55 {
		t.Errorf("allowed %d, expected at most ~50", trueCount)
	}
	if trueCount < 1 {
		t.Error("should allow at least 1 request")
	}
}

// --- Tiered limiter construction ---

func TestNewTieredLimiters(t *testing.T) {
	tl := newTieredLimiters()

	if tl.anon == nil || tl.authed == nil || tl.agent == nil {
		t.Fatal("all tiers should be initialized")
	}

	if tl.anon.burst != anonBurst {
		t.Errorf("anon burst = %d, want %d", tl.anon.burst, anonBurst)
	}
	if tl.authed.burst != authedBurst {
		t.Errorf("authed burst = %d, want %d", tl.authed.burst, authedBurst)
	}
	if tl.agent.burst != agentBurst {
		t.Errorf("agent burst = %d, want %d", tl.agent.burst, agentBurst)
	}
}

func TestTieredLimiters_IndependentBuckets(t *testing.T) {
	tl := newTieredLimiters()

	// Exhaust anon tier
	for range anonBurst {
		tl.anon.allow("192.168.1.1")
	}
	if tl.anon.allow("192.168.1.1") {
		t.Error("anon should be exhausted")
	}

	// Authed tier for same IP should still work
	if !tl.authed.allow("192.168.1.1") {
		t.Error("authed tier should be independent from anon")
	}

	// Agent tier for same IP should still work
	if !tl.agent.allow("192.168.1.1") {
		t.Error("agent tier should be independent from anon")
	}
}

// --- Middleware tests ---

func TestClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"

	s, _, _, _ := newTestServer()
	got := s.clientIP(req)
	if got != "10.0.0.1" {
		t.Errorf("clientIP = %s, want 10.0.0.1", got)
	}
}

func TestRateLimit_AnonMiddleware(t *testing.T) {
	s, _, _, _ := newTestServer()
	s.Limiters = newTieredLimiters()
	s.Limiters.anon = newRateLimiter(100, 2)

	handler := s.rateLimit(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for i := range 2 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("request %d: got %d, want 200", i+1, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("request 3: got %d, want 429", rec.Code)
	}
}

func TestRateLimitAuthed_KeysByUsername(t *testing.T) {
	s, _, _, _ := newTestServer()
	s.Limiters = newTieredLimiters()
	s.Limiters.authed = newRateLimiter(100, 2)

	handler := s.rateLimitAuthed(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Two requests from user "alice" on different IPs should share a bucket
	for i := range 2 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if i == 0 {
			req.RemoteAddr = "10.0.0.1:1234"
		} else {
			req.RemoteAddr = "10.0.0.2:1234"
		}
		ctx := contextWithUser(req.Context(), &userContext{Username: "alice", Role: "admin"})
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("request %d: got %d, want 200", i+1, rec.Code)
		}
	}

	// Third from alice (any IP) should be blocked
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.3:1234"
	ctx := contextWithUser(req.Context(), &userContext{Username: "alice", Role: "admin"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("request 3: got %d, want 429", rec.Code)
	}
}

func TestRateLimitAuthed_DifferentUsersSeparate(t *testing.T) {
	s, _, _, _ := newTestServer()
	s.Limiters = newTieredLimiters()
	s.Limiters.authed = newRateLimiter(100, 1)

	handler := s.rateLimitAuthed(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Exhaust alice's bucket
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	ctx := contextWithUser(req.Context(), &userContext{Username: "alice", Role: "admin"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("alice request 1: got %d, want 200", rec.Code)
	}

	// Bob from same IP should still work
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	ctx = contextWithUser(req.Context(), &userContext{Username: "bob", Role: "viewer"})
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("bob request 1: got %d, want 200", rec.Code)
	}
}

func TestRateLimitAuthed_FallsBackToIP(t *testing.T) {
	s, _, _, _ := newTestServer()
	s.Limiters = newTieredLimiters()
	s.Limiters.authed = newRateLimiter(100, 1)

	handler := s.rateLimitAuthed(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// No user context — should key on IP
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("request 1: got %d, want 200", rec.Code)
	}

	// Second from same IP, no user context — should be blocked
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec = httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("request 2: got %d, want 429", rec.Code)
	}
}

func TestRateLimitAgent_Middleware(t *testing.T) {
	s, _, _, _ := newTestServer()
	s.Limiters = newTieredLimiters()
	s.Limiters.agent = newRateLimiter(100, 2)

	handler := s.rateLimitAgent(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for i := range 2 {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.RemoteAddr = "10.0.0.5:1234"
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("request %d: got %d, want 200", i+1, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "10.0.0.5:1234"
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("request 3: got %d, want 429", rec.Code)
	}
}

// --- Benchmark ---

func BenchmarkRateLimiter_Allow(b *testing.B) {
	rl := newRateLimiter(1000, 100)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		rl.allow("192.168.1.1")
	}
}

func TestParseTrustedProxies(t *testing.T) {
	prefixes, invalid := parseTrustedProxies([]string{
		"198.51.100.0/24",
		"203.0.113.7",
		"  192.0.2.0/25  ",
		"2001:db8::/32",
		"",
		"not-an-address",
		"10.0.0.0/99",
	})

	if len(prefixes) != 4 {
		t.Errorf("parsed prefixes: got %d, want 4", len(prefixes))
	}
	if len(invalid) != 2 {
		t.Errorf("invalid entries: got %v, want 2", invalid)
	}
}

func TestIsTrustedProxy(t *testing.T) {
	s, _, _, _ := newTestServer()
	s.trustedProxies, _ = parseTrustedProxies([]string{"198.51.100.0/24", "203.0.113.7", "2001:db8::/32"})

	tests := []struct {
		addr string
		want bool
	}{
		{"198.51.100.1", true},
		{"198.51.100.255", true},
		{"203.0.113.7", true},
		{"203.0.113.8", false},
		{"192.0.2.1", false},
		{"2001:db8::1", true},
		{"2001:db9::1", false},
		{"::ffff:198.51.100.1", true}, // v4-mapped must not read as untrusted
	}

	for _, tc := range tests {
		t.Run(tc.addr, func(t *testing.T) {
			addr, err := netip.ParseAddr(tc.addr)
			if err != nil {
				t.Fatalf("parse %q: %v", tc.addr, err)
			}
			if got := s.isTrustedProxy(addr); got != tc.want {
				t.Errorf("isTrustedProxy(%s): got %v, want %v", tc.addr, got, tc.want)
			}
		})
	}
}

// TestClientIP_IgnoresForwardedHeadersFromUntrustedPeer is the regression test
// for the original bug: any caller could set X-Forwarded-For to a new value on
// every request and get a fresh login-lockout and rate-limit bucket each time.
func TestClientIP_IgnoresForwardedHeadersFromUntrustedPeer(t *testing.T) {
	s, _, _, _ := newTestServer()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	req.RemoteAddr = "203.0.113.50:44444"
	req.Header.Set("X-Forwarded-For", "192.0.2.1")
	req.Header.Set("X-Real-IP", "192.0.2.2")

	if got := s.clientIP(req); got != "203.0.113.50" {
		t.Errorf("clientIP: got %q, want the peer address 203.0.113.50", got)
	}
}

func TestClientIP_NoTrustedProxiesConfigured(t *testing.T) {
	s, _, _, _ := newTestServer()
	if len(s.trustedProxies) != 0 {
		t.Fatalf("expected no trusted proxies by default, got %v", s.trustedProxies)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	req.RemoteAddr = "198.51.100.9:1234"
	req.Header.Set("X-Forwarded-For", "192.0.2.1")

	if got := s.clientIP(req); got != "198.51.100.9" {
		t.Errorf("clientIP: got %q, want 198.51.100.9", got)
	}
}

func TestClientIP_TrustedProxy(t *testing.T) {
	s, _, _, _ := newTestServer()
	s.trustedProxies, _ = parseTrustedProxies([]string{"198.51.100.0/24"})

	tests := []struct {
		name string
		xff  string
		xri  string
		want string
	}{
		{
			name: "single hop",
			xff:  "192.0.2.1",
			want: "192.0.2.1",
		},
		{
			name: "stops at the first untrusted hop from the right",
			xff:  "192.0.2.1, 203.0.113.9, 198.51.100.20",
			want: "203.0.113.9",
		},
		{
			name: "a client-supplied prefix cannot displace the real hop",
			xff:  "10.9.9.9, 192.0.2.1, 198.51.100.20",
			want: "192.0.2.1",
		},
		{
			name: "unparseable entries are skipped",
			xff:  "192.0.2.1, not-an-address, 198.51.100.20",
			want: "192.0.2.1",
		},
		{
			name: "falls back to X-Real-IP when there is no XFF",
			xri:  "192.0.2.5",
			want: "192.0.2.5",
		},
		{
			name: "every hop trusted falls back to the peer",
			xff:  "198.51.100.21, 198.51.100.20",
			want: "198.51.100.10",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
			req.RemoteAddr = "198.51.100.10:44444"
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			if tc.xri != "" {
				req.Header.Set("X-Real-IP", tc.xri)
			}

			if got := s.clientIP(req); got != tc.want {
				t.Errorf("clientIP: got %q, want %q", got, tc.want)
			}
		})
	}
}
