package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func newRecordingTracer() (trace.Tracer, *tracetest.SpanRecorder) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	return tp.Tracer("test"), sr
}

// Composed the way Start composes it, so a change that moves traceRequests
// outside requestLogger loses the route name and fails here.
func TestTraceRequests_NamesSpanByRoute(t *testing.T) {
	s, _, _, _ := newTestServer()
	tr, sr := newRecordingTracer()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/agents/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	h := s.requestLogger(traceRequests(mux, tr))

	agentID := "00000000-0000-4000-8000-000000000001"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+agentID, nil)
	req.Header.Set("X-Agent-ID", agentID)
	h.ServeHTTP(httptest.NewRecorder(), req)

	ended := sr.Ended()
	if len(ended) != 1 {
		t.Fatalf("ended %d spans, want 1", len(ended))
	}
	span := ended[0]

	if got, want := span.Name(), "GET /api/v1/agents/{id}"; got != want {
		t.Errorf("span name = %q, want %q", got, want)
	}
	if span.SpanKind() != trace.SpanKindServer {
		t.Errorf("span kind = %v, want server", span.SpanKind())
	}

	attrs := attribute.NewSet(span.Attributes()...)
	if v, _ := attrs.Value("http.route"); v.AsString() != "/api/v1/agents/{id}" {
		t.Errorf("http.route = %q, want /api/v1/agents/{id}", v.AsString())
	}
	if v, _ := attrs.Value("http.response.status_code"); v.AsInt64() != http.StatusTeapot {
		t.Errorf("http.response.status_code = %d, want %d", v.AsInt64(), http.StatusTeapot)
	}
	if v, _ := attrs.Value("spectra.agent_id"); v.AsString() != agentID {
		t.Errorf("spectra.agent_id = %q, want %q", v.AsString(), agentID)
	}
}

func TestTraceRequests_SkipsLongPollAndFrontend(t *testing.T) {
	tr, sr := newRecordingTracer()
	h := traceRequests(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), tr)

	for _, path := range []string{commandPollPath, "/", "/assets/index.js"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}

	if n := len(sr.Started()); n != 0 {
		t.Errorf("started %d spans, want 0", n)
	}
}

func TestTraceRequests_StatusCode(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   codes.Code
	}{
		{"success", http.StatusAccepted, codes.Unset},
		{"client error", http.StatusUnauthorized, codes.Unset},
		{"server error", http.StatusInternalServerError, codes.Error},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr, sr := newRecordingTracer()
			mux := http.NewServeMux()
			mux.HandleFunc("POST /api/v1/agent/metrics", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			})

			req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/metrics", nil)
			traceRequests(mux, tr).ServeHTTP(httptest.NewRecorder(), req)

			ended := sr.Ended()
			if len(ended) != 1 {
				t.Fatalf("ended %d spans, want 1", len(ended))
			}
			if got := ended[0].Status().Code; got != tt.want {
				t.Errorf("status = %v, want %v", got, tt.want)
			}
		})
	}
}
