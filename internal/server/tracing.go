package server

import (
	"net/http"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// tracer opens the spans handlers add under the request span. It resolves to a
// no-op until telemetry.Setup installs a provider.
var tracer = otel.Tracer("github.com/nhdewitt/spectra/internal/server")

// commandPollPath is not traced. Every request holds for the full wait by design.
const commandPollPath = "/api/v1/agent/command"

// traceRequests opens a server span for each API request, named by its route.
//
// It must wrap the Router directly. requestLogger passes the next handler a copy
// of the request, and ServeMux records the matched pattern on the copy it is given,
// so a wrapper any further out never sees r.Pattern.
func traceRequests(next http.Handler, tr trace.Tracer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == commandPollPath {
			next.ServeHTTP(w, r)
			return
		}

		ctx, span := tr.Start(r.Context(), r.Method, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		r = r.WithContext(ctx)
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		span.SetAttributes(
			attribute.String("http.request.method", r.Method),
			attribute.Int("http.response.status_code", sw.status),
		)

		if r.Pattern != "" {
			route := r.Pattern
			if _, path, ok := strings.Cut(route, " "); ok {
				route = path
			}
			span.SetName(r.Method + " " + route)
			span.SetAttributes(attribute.String("http.route", route))
		}

		if id := getAgentID(r); id != "" {
			span.SetAttributes(attribute.String("spectra.agent_id", id))
		}
		if sw.status >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, http.StatusText(sw.status))
		}
	})
}
