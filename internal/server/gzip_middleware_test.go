package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// echoHandler writes a body large enough that compression is observable.
func echoHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"padding":"` + strings.Repeat("a", 4096) + `"}`))
	})
}

func TestGzipMiddleware_CompressesAPIResponses(t *testing.T) {
	h := gzipMiddleware(echoHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if enc := rec.Header().Get("Content-Encoding"); enc != "gzip" {
		t.Fatalf("Content-Encoding: got %q, want gzip", enc)
	}
	if vary := rec.Header().Get("Vary"); vary != "Accept-Encoding" {
		t.Errorf("Vary: got %q, want Accept-Encoding", vary)
	}

	gz, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("response body is not valid gzip: %v", err)
	}
	defer gz.Close()

	body, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("read decompressed body: %v", err)
	}
	if !strings.HasPrefix(string(body), `{"padding":"aaa`) {
		t.Errorf("decompressed body does not match what the handler wrote")
	}
	if rec.Body.Len() >= len(body) {
		t.Errorf("compressed body (%d) is not smaller than the original (%d)", rec.Body.Len(), len(body))
	}
}

func TestGzipMiddleware_SkipsWithoutAcceptEncoding(t *testing.T) {
	h := gzipMiddleware(echoHandler())

	// curl sends no Accept-Encoding by default, which is why manual curl
	// testing never reproduced the release-download corruption below.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if enc := rec.Header().Get("Content-Encoding"); enc != "" {
		t.Errorf("Content-Encoding: got %q, want empty", enc)
	}
	if !strings.HasPrefix(rec.Body.String(), `{"padding":"aaa`) {
		t.Error("body was altered for a client that did not ask for gzip")
	}
}

// TestGzipMiddleware_SkipsFrontend covers the original exclusion: the embedded
// frontend is served with its own Content-Length, and compressing the body
// after that length is set makes the two disagree.
func TestGzipMiddleware_SkipsFrontend(t *testing.T) {
	h := gzipMiddleware(echoHandler())

	for _, path := range []string{"/", "/index.html", "/assets/index-abc123.js"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Accept-Encoding", "gzip")
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if enc := rec.Header().Get("Content-Encoding"); enc != "" {
				t.Errorf("Content-Encoding: got %q, want empty", enc)
			}
		})
	}
}

// TestGzipMiddleware_SkipsReleaseDownloads is the regression test for the agent
// self-update corruption. handleDownloadRelease sets its own Content-Length
// from the file size, and ServeContent skips correcting it when
// Content-Encoding is already set -- which this middleware does before the
// handler runs. The advertised length was the uncompressed size while the bytes
// sent were the compressed ones, so the connection closed early and the agent
// reported "download write failed: unexpected EOF" after nearly a full
// download. Agents send Accept-Encoding: gzip on every request, so this fired
// on every self-update.
func TestGzipMiddleware_SkipsReleaseDownloads(t *testing.T) {
	h := gzipMiddleware(echoHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/releases/spectra-agent-linux-amd64", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if enc := rec.Header().Get("Content-Encoding"); enc != "" {
		t.Fatalf("Content-Encoding: got %q, want empty: compressing a response that carries its own Content-Length truncates it", enc)
	}
	if !strings.HasPrefix(rec.Body.String(), `{"padding":"aaa`) {
		t.Error("release download body was altered")
	}
}

// TestGzipMiddleware_PreservesStatusCode guards the embedding: Header and
// WriteHeader come from the wrapped ResponseWriter, so only the body is
// compressed.
func TestGzipMiddleware_PreservesStatusCode(t *testing.T) {
	h := gzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test-Header", "test-value")
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("short"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Errorf("status: got %d, want 418", rec.Code)
	}
	if got := rec.Header().Get("X-Test-Header"); got != "test-value" {
		t.Errorf("X-Test-Header: got %q, want test-value", got)
	}
}

// TestGzipMiddleware_ReusedWriterProducesIndependentStreams exercises the
// sync.Pool path: a writer returned to the pool and picked up by a later
// request must not carry any of the previous body with it.
func TestGzipMiddleware_ReusedWriterProducesIndependentStreams(t *testing.T) {
	h := gzipMiddleware(echoHandler())

	for i := range 3 {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		gz, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
		if err != nil {
			t.Fatalf("request %d: body is not valid gzip: %v", i, err)
		}
		body, err := io.ReadAll(gz)
		gz.Close()
		if err != nil {
			t.Fatalf("request %d: read body: %v", i, err)
		}
		if len(body) != len(`{"padding":""}`)+4096 {
			t.Errorf("request %d: decompressed %d bytes, want %d", i, len(body), len(`{"padding":""}`)+4096)
		}
	}
}
