package server

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

const (
	testViewerToken = "test-viewer-session-token"
	testViewerIP    = "198.51.100.7"
)

// alertMutationRoutes is every alert route that changes state. A viewer must be
// refused on all of them: creating a webhook channel plus a rule that fires it
// turns a read-only account into a request generator running inside the
// server's network.
var alertMutationRoutes = []struct {
	name   string
	method string
	path   string
	body   string
}{
	{
		"create channel", http.MethodPost, "/api/v1/alerts/channels",
		`{"name":"test-channel","type":"webhook","config":{"url":"https://webhook.example/hook"}}`,
	},
	{
		"update channel", http.MethodPut, "/api/v1/alerts/channels/" + testAgentUUID,
		`{"name":"test-channel","type":"webhook","config":{"url":"https://webhook.example/hook"}}`,
	},
	{"delete channel", http.MethodDelete, "/api/v1/alerts/channels/" + testAgentUUID, ""},
	{
		"create rule", http.MethodPost, "/api/v1/alerts/rules",
		`{"name":"test-rule","condition_type":"agent_offline","scope":"global","params":{"timeout_seconds":300}}`,
	},
	{
		"update rule", http.MethodPut, "/api/v1/alerts/rules/" + testAgentUUID,
		`{"name":"test-rule","condition_type":"agent_offline","scope":"global","params":{"timeout_seconds":300}}`,
	},
	{
		"toggle rule", http.MethodPut, "/api/v1/alerts/rules/" + testAgentUUID + "/enabled",
		`{"enabled":true}`,
	},
	{"delete rule", http.MethodDelete, "/api/v1/alerts/rules/" + testAgentUUID, ""},
}

func TestAlertMutations_RejectViewer(t *testing.T) {
	for _, tc := range alertMutationRoutes {
		t.Run(tc.name, func(t *testing.T) {
			s, _, _, mock := newTestServer()
			setupTestSessionWithRole(mock, testViewerToken, "test-viewer", RoleViewer, testViewerIP, pgtype.UUID{})

			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req = authedRequestAs(req, testViewerToken, testViewerIP)

			rec := httptest.NewRecorder()
			s.Router.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Errorf("%s %s as viewer: got %d, want 403", tc.method, tc.path, rec.Code)
			}
		})
	}
}

// TestAlertMutations_AllowAdmin is the positive control for the test above: it
// fails if the routes get blanket-denied rather than role-gated.
func TestAlertMutations_AllowAdmin(t *testing.T) {
	for _, tc := range alertMutationRoutes {
		t.Run(tc.name, func(t *testing.T) {
			s, _, _, mock := newTestServer()
			setupTestSession(mock)

			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req = authedRequest(req)

			rec := httptest.NewRecorder()
			s.Router.ServeHTTP(rec, req)

			if rec.Code == http.StatusForbidden {
				t.Errorf("%s %s as admin: got 403, want it allowed through", tc.method, tc.path)
			}
		})
	}
}

// TestAlertReads_AllowViewer pins the other half of the boundary: the read
// routes stay open to viewers.
func TestAlertReads_AllowViewer(t *testing.T) {
	paths := []string{
		"/api/v1/alerts/channels",
		"/api/v1/alerts/rules",
		"/api/v1/alerts/active",
		"/api/v1/alerts/history",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			s, _, _, mock := newTestServer()
			setupTestSessionWithRole(mock, testViewerToken, "test-viewer", RoleViewer, testViewerIP, pgtype.UUID{})

			req := authedRequestAs(httptest.NewRequest(http.MethodGet, path, nil), testViewerToken, testViewerIP)
			rec := httptest.NewRecorder()
			s.Router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("GET %s as viewer: got %d, want 200", path, rec.Code)
			}
		})
	}
}

func TestValidateWebhookURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"public https", "https://webhook.example/hook", false},
		{"lan literal is allowed", "http://198.51.100.20:9000/hook", false},
		{"hostname is deferred to dial time", "http://internal-bridge.test/hook", false},
		{"loopback v4", "http://127.0.0.1:8080/hook", true},
		{"loopback name is not resolved here", "http://localhost:8080/hook", false},
		{"loopback v6", "http://[::1]:8080/hook", true},
		{"loopback v4-mapped v6", "http://[::ffff:127.0.0.1]:8080/hook", true},
		{"link-local metadata", "http://169.254.169.254/latest/meta-data/", true},
		{"unspecified", "http://0.0.0.0:8080/hook", true},
		{"file scheme", "file:///etc/spectra/server.json", true},
		{"gopher scheme", "gopher://webhook.example/1", true},
		{"no host", "https:///hook", true},
		{"embedded credentials", "https://user:pass@webhook.example/hook", true},
		{"empty", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateWebhookURL(tc.url)
			if tc.wantErr && err == nil {
				t.Errorf("validateWebhookURL(%q): got nil, want error", tc.url)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("validateWebhookURL(%q): got %v, want nil", tc.url, err)
			}
		})
	}
}

func TestWebhookAddrAllowed(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		{"203.0.113.10", true},
		{"198.51.100.20", true},
		{"10.10.107.1", true}, // RFC1918 is deliberately permitted
		{"fd00::1", true},     // ULA, same rationale
		{"100.64.0.5", true},  // CGNAT/tailnet
		{"127.0.0.1", false},
		{"127.0.0.53", false},
		{"::1", false},
		{"::ffff:127.0.0.1", false},
		{"169.254.169.254", false},
		{"fe80::1", false},
		{"0.0.0.0", false},
		{"::", false},
		{"224.0.0.1", false},
		{"255.255.255.255", false},
	}

	for _, tc := range tests {
		t.Run(tc.addr, func(t *testing.T) {
			ip, err := netip.ParseAddr(tc.addr)
			if err != nil {
				t.Fatalf("parse %q: %v", tc.addr, err)
			}
			if got := webhookAddrAllowed(ip); got != tc.want {
				t.Errorf("webhookAddrAllowed(%s): got %v, want %v", tc.addr, got, tc.want)
			}
		})
	}
}

// TestWebhookClient_BlocksLoopbackDial covers what URL validation cannot: the
// dialer refuses a blocked address even when the URL passed validation, which
// is what closes DNS rebinding and redirect-to-loopback.
func TestWebhookClient_BlocksLoopbackDial(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := newWebhookClient().Do(req)
	if err == nil {
		resp.Body.Close()
		t.Fatal("dialed a loopback webhook target, want it refused")
	}
	if !strings.Contains(err.Error(), errWebhookTargetBlocked.Error()) {
		t.Errorf("error: got %v, want it to mention %v", err, errWebhookTargetBlocked)
	}
}
