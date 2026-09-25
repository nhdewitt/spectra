package telemetry

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestQueryName(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want string
	}{
		{"sqlc exec", "-- name: InsertCPU :exec\nINSERT INTO metrics_cpu (time) VALUES ($1)", "InsertCPU"},
		{"sqlc one", "-- name: GetAgent :one\nSELECT id FROM agents WHERE id = $1", "GetAgent"},
		{"transaction begin", "begin", "begin"},
		{"transaction commit", "commit", "commit"},
		{"hand-written, leading whitespace", "\n\tSELECT count(*) FROM agents", "SELECT"},
		{"keyword followed by paren", "SELECT(1)", "SELECT"},
		{"empty", "", "query"},
		{"whitespace only", " \n\t", "query"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := queryName(tt.sql); got != tt.want {
				t.Errorf("queryName(%q) = %q, want %q", tt.sql, got, tt.want)
			}
		})
	}
}

func newRecordingTracer() (*PgxTracer, trace.Tracer, *tracetest.SpanRecorder) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	return &PgxTracer{tracer: tp.Tracer("test")}, tp.Tracer("test"), sr
}

func TestPgxTracer_NoParentNoSpan(t *testing.T) {
	pt, _, sr := newRecordingTracer()
	ctx := context.Background()

	qctx := pt.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: "-- name: GetAgent :one\nSELECT 1"})
	if qctx != ctx {
		t.Error("TraceQueryStart replaced a context that carried no span")
	}
	pt.TraceQueryEnd(qctx, nil, pgx.TraceQueryEndData{})

	actx := pt.TraceAcquireStart(ctx, nil, pgxpool.TraceAcquireStartData{})
	pt.TraceAcquireEnd(actx, nil, pgxpool.TraceAcquireEndData{})

	if n := len(sr.Started()); n != 0 {
		t.Errorf("started %d spans without a parent, want 0", n)
	}
}

func TestPgxTracer_ChildOfRequestSpan(t *testing.T) {
	pt, tr, sr := newRecordingTracer()
	ctx, parent := tr.Start(context.Background(), "POST /api/v1/agent/metrics")

	actx := pt.TraceAcquireStart(ctx, nil, pgxpool.TraceAcquireStartData{})
	pt.TraceAcquireEnd(actx, nil, pgxpool.TraceAcquireEndData{})

	qctx := pt.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: "-- name: InsertCPU :exec\nINSERT INTO metrics_cpu (time) VALUES ($1)"})
	pt.TraceQueryEnd(qctx, nil, pgx.TraceQueryEndData{})

	// Ending a query must never end the request span it hangs from.
	if !parent.IsRecording() {
		t.Fatal("parent span ended by a child's End")
	}
	parent.End()

	ended := sr.Ended()
	if len(ended) != 3 {
		t.Fatalf("ended %d spans, want 3", len(ended))
	}

	want := []string{"pool.acquire", "InsertCPU"}
	for i, name := range want {
		s := ended[i]
		if s.Name() != name {
			t.Errorf("span %d name = %q, want %q", i, s.Name(), name)
		}
		if s.Parent().SpanID() != parent.SpanContext().SpanID() {
			t.Errorf("span %q is not a child of the request span", s.Name())
		}
		if s.SpanKind() != trace.SpanKindClient {
			t.Errorf("span %q kind = %v, want client", s.Name(), s.SpanKind())
		}
	}
}

func TestPgxTracer_ErrorStatus(t *testing.T) {
	pt, tr, sr := newRecordingTracer()
	ctx, parent := tr.Start(context.Background(), "request")
	defer parent.End()

	qctx := pt.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: "commit"})
	pt.TraceQueryEnd(qctx, nil, pgx.TraceQueryEndData{Err: errors.New("connection reset")})

	ended := sr.Ended()
	if len(ended) != 1 {
		t.Fatalf("ended %d spans, want 1", len(ended))
	}
	if got := ended[0].Status().Code; got != codes.Error {
		t.Errorf("status = %v, want Error", got)
	}
}

func TestPgxTracer_RowsAffected(t *testing.T) {
	pt, tr, sr := newRecordingTracer()
	ctx, parent := tr.Start(context.Background(), "request")
	defer parent.End()

	qctx := pt.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: "-- name: UpsertProcesses :exec\nINSERT INTO current_processes (pid) VALUES (1)"})
	pt.TraceQueryEnd(qctx, nil, pgx.TraceQueryEndData{CommandTag: pgconn.NewCommandTag("INSERT 0 3")})

	ended := sr.Ended()
	if len(ended) != 1 {
		t.Fatalf("ended %d spans, want 1", len(ended))
	}
	attrs := attribute.NewSet(ended[0].Attributes()...)
	if v, _ := attrs.Value("db.rows_affected"); v.AsInt64() != 3 {
		t.Errorf("db.rows_affected = %d, want 3", v.AsInt64())
	}
}
