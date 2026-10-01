package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The command long-poll must not inherit the per-request deadline, or it ends
// before Config.CommandTimeout.
func TestRequestLogger_Deadline(t *testing.T) {
	s, _, _, _ := newTestServer()

	cases := []struct {
		path         string
		wantDeadline bool
	}{
		{commandPollPath, false},
		{"/api/v1/agent/metrics", true},
		{"/api/v1/agent/command/result", true},
		{"/api/v1/overview", true},
	}
	for _, c := range cases {
		var deadline time.Time
		var has bool
		h := s.requestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			deadline, has = r.Context().Deadline()
		}))
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, c.path, nil))

		if has != c.wantDeadline {
			t.Errorf("%s: has deadline = %v, want %v", c.path, has, c.wantDeadline)
			continue
		}
		if has {
			if left := time.Until(deadline); left <= 0 || left > requestTimeout {
				t.Errorf("%s: deadline in %v, want within %v", c.path, left, requestTimeout)
			}
		}
	}
}
