package server

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	// requestTimeout bounds every request except the command long-poll, which
	// CommandQueue.Wait bounds with Config.CommandTimeout.
	requestTimeout = 15 * time.Second
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}

func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != commandPollPath {
			ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
			defer cancel()
			r = r.WithContext(ctx)
		}

		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)

		duration := time.Since(start)
		level := slog.LevelDebug

		isAgentPoll := strings.HasPrefix(r.URL.Path, "/api/v1/agent/")
		if isAgentPoll && sw.status < 400 {
			return
		}

		if !isAgentPoll && duration > 500*time.Millisecond {
			level = slog.LevelInfo
		}
		if sw.status >= 400 {
			level = slog.LevelWarn
		}
		if sw.status >= 500 {
			level = slog.LevelError
		}

		s.Logger.Log(r.Context(), level, "request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", duration.Milliseconds())
	})
}
