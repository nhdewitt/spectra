package pprofd

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestServer(t *testing.T) (*Server, *Store) {
	t.Helper()

	store, err := NewStore(t.TempDir(), 0)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewServer(store, log), store
}

func postUpload(t *testing.T, s *Server, up Upload) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(up)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/profiles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

func TestServer_UploadThenList(t *testing.T) {
	s, _ := newTestServer(t)
	at := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)

	rec := postUpload(t, s, testUpload("agent-1", "cpu", at))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("upload status = %d, want 202: %s", rec.Code, rec.Body)
	}

	var created map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal upload response: %v", err)
	}
	if created["id"] == "" {
		t.Fatal("upload response carried no id")
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/agent-1/profiles", nil)
	listed := httptest.NewRecorder()
	s.ServeHTTP(listed, req)

	if listed.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", listed.Code)
	}

	var recs []Record
	if err := json.Unmarshal(listed.Body.Bytes(), &recs); err != nil {
		t.Fatalf("unmarshal listing: %v", err)
	}
	if len(recs) != 1 || recs[0].ID != created["id"] {
		t.Fatalf("listing = %+v, want the uploaded record", recs)
	}
	if recs[0].Runtime.GOMEMLIMIT == 0 {
		t.Error("runtime snapshot did not survive the round trip")
	}
}

// go tool pprof fetches this URL directly, so it has to be raw profile bytes
// rather than the JSON envelope.
func TestServer_ProfileServesRawBytes(t *testing.T) {
	s, _ := newTestServer(t)

	rec := postUpload(t, s, testUpload("agent-1", "cpu", time.Now()))
	var created map[string]string
	json.Unmarshal(rec.Body.Bytes(), &created)

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/agent-1/profiles/"+created["id"], nil)
	got := httptest.NewRecorder()
	s.ServeHTTP(got, req)

	if got.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", got.Code)
	}
	if ct := got.Header().Get("Content-Type"); ct != "application/octet-stream" {
		t.Errorf("Content-Type = %q", ct)
	}
	if !bytes.Equal(got.Body.Bytes(), []byte{0x1f, 0x8b, 0x01, 0x02}) {
		t.Errorf("body = % x, want the raw profile bytes", got.Body.Bytes())
	}
}

// go tool pprof appends nothing, but a browser download and a re-upload round
// trip both tend to carry the extension.
func TestServer_ProfileAcceptsPprofSuffix(t *testing.T) {
	s, _ := newTestServer(t)

	rec := postUpload(t, s, testUpload("agent-1", "heap", time.Now()))
	var created map[string]string
	json.Unmarshal(rec.Body.Bytes(), &created)

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/agent-1/profiles/"+created["id"]+".pprof", nil)
	got := httptest.NewRecorder()
	s.ServeHTTP(got, req)

	if got.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", got.Code)
	}
}

func TestServer_RejectsMalformedUpload(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/profiles", strings.NewReader("{not json"))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestServer_RejectsUploadWithoutProfile(t *testing.T) {
	s, _ := newTestServer(t)

	up := testUpload("agent-1", "cpu", time.Now())
	up.Profile = nil

	if rec := postUpload(t, s, up); rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestServer_UnknownAgentIs404(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/agent-404/profiles", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestServer_UnknownProfileIs404(t *testing.T) {
	s, _ := newTestServer(t)
	postUpload(t, s, testUpload("agent-1", "cpu", time.Now()))

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/agent-1/profiles/20000101T000000.000-cpu", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestServer_AgentsIsEmptyArrayNotNull(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/agents", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if body := strings.TrimSpace(rec.Body.String()); body != "[]" {
		t.Errorf("body = %s, want []", body)
	}
}

func TestServer_Health(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestServer_IndexListsAgents(t *testing.T) {
	s, _ := newTestServer(t)
	postUpload(t, s, testUpload("agent-1", "cpu", time.Now()))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	body := rec.Body.String()
	for _, want := range []string{"test-host", "Test CPU", "linux/arm64"} {
		if !strings.Contains(body, want) {
			t.Errorf("index does not mention %q", want)
		}
	}
}

// Hostname and CPU model come off the wire, so the index must not render them
// as markup.
func TestServer_IndexEscapesHostFacts(t *testing.T) {
	s, _ := newTestServer(t)

	up := testUpload("agent-1", "cpu", time.Now())
	up.Hostname = `<script>alert(1)</script>`
	postUpload(t, s, up)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	body := rec.Body.String()

	// The page carries its own <script> for the copy buttons, so look for the
	// injected payload rather than for a script tag.
	if strings.Contains(body, "<script>alert(1)") {
		t.Error("hostname was rendered unescaped")
	}
	if !strings.Contains(body, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Error("hostname was not escaped into the page at all")
	}
}

func TestServer_UnknownPathIs404(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/nope", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestServer_SummaryEndpoint(t *testing.T) {
	s, _ := newTestServer(t)
	postUpload(t, s, testUpload("agent-1", "cpu", time.Now()))

	req := httptest.NewRequest(http.MethodGet, "/v1/summary", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var sums []Summary
	if err := json.Unmarshal(rec.Body.Bytes(), &sums); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(sums) != 1 || sums[0].Hostname != "test-host" {
		t.Fatalf("summaries = %+v", sums)
	}
	if sums[0].GCCPUFraction != nil {
		t.Error("a rate was reported from a single capture")
	}
}

func TestServer_SummaryIsEmptyArrayNotNull(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/summary", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if body := strings.TrimSpace(rec.Body.String()); body != "[]" {
		t.Errorf("body = %s, want []", body)
	}
}

func TestServer_IndexShowsComparisonTable(t *testing.T) {
	s, _ := newTestServer(t)

	for _, host := range []string{"pi", "amd64box"} {
		up := testUpload("agent-"+host, "cpu", time.Now())
		up.Hostname = host
		postUpload(t, s, up)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Across agents") {
		t.Error("index has no comparison table")
	}
	for _, host := range []string{"pi", "amd64box"} {
		if !strings.Contains(body, host) {
			t.Errorf("comparison table omits %q", host)
		}
	}
}

func TestServer_IndexHasCopyButtons(t *testing.T) {
	s, _ := newTestServer(t)
	postUpload(t, s, testUpload("agent-1", "cpu", time.Now()))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "button class=copy") {
		t.Error("no copy button rendered")
	}
	if !strings.Contains(body, "go tool pprof") {
		t.Error("copy script does not build a pprof command")
	}
}

// A heap profile carries four sample types in one file, and the default view
// is rarely the useful one.
func TestCopyButtons_HeapOffersEverySampleIndex(t *testing.T) {
	got := copyButtons("heap")

	for _, want := range []string{"inuse_space", "inuse_objects", "alloc_space", "alloc_objects"} {
		if !strings.Contains(got, want) {
			t.Errorf("heap row has no button for %s", want)
		}
	}
}

func TestCopyButtons_OtherKindsGetOneButton(t *testing.T) {
	for _, kind := range []string{"cpu", "goroutine"} {
		got := copyButtons(kind)

		if n := strings.Count(got, "<button"); n != 1 {
			t.Errorf("%s row rendered %d buttons, want 1", kind, n)
		}
		if strings.Contains(got, "sample_index") {
			t.Errorf("%s row offers a sample index it has no use for", kind)
		}
	}
}

// -http=: has to run on the caller's machine, so the page can only offer it as
// a modifier on the copied command. A per-row variant would double four heap
// buttons to eight.
func TestServer_IndexHasBrowserModeToggle(t *testing.T) {
	s, _ := newTestServer(t)
	postUpload(t, s, testUpload("agent-1", "heap", time.Now()))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `id=httpmode`) {
		t.Error("no browser-mode toggle rendered")
	}
	if !strings.Contains(body, `-http=:`) {
		t.Error("copy script never adds -http=:")
	}
}

func TestServer_IndexHeapRowCarriesSampleIndexFlags(t *testing.T) {
	s, _ := newTestServer(t)
	postUpload(t, s, testUpload("agent-1", "heap", time.Now()))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), "-sample_index=alloc_space") {
		t.Error("heap row does not offer the alloc_space view")
	}
}

// navigator.clipboard is undefined over plain HTTP, which is how this is
// served, so the execCommand path has to be present.
func TestServer_CopyScriptHasInsecureContextPath(t *testing.T) {
	s, _ := newTestServer(t)
	postUpload(t, s, testUpload("agent-1", "cpu", time.Now()))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	body := rec.Body.String()
	for _, want := range []string{"isSecureContext", "execCommand"} {
		if !strings.Contains(body, want) {
			t.Errorf("copy script is missing %q", want)
		}
	}
}
