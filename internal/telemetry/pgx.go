package telemetry

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// dbSystem is set on every span PgxTracer opens.
var dbSystem = attribute.String("db.system.name", "postgresql")

// PgxTracer opens a client span for each query and each pool acquire, as a child
// of the span already in the query's context.
//
// Queries whose context carries no span are not traced. That covers migrations,
// the alert evaluator, and session cleanup which would otherwise produce a steady
// stream of single-span traces with no request to attach them to.
type PgxTracer struct {
	tracer trace.Tracer
}

var (
	_ pgx.QueryTracer       = (*PgxTracer)(nil)
	_ pgxpool.AcquireTracer = (*PgxTracer)(nil)
)

// NewPgxTracer returns a tracer for pgxpool.Config.ConnConfig.Tracer. pgxpool
// detects the AcquireTracer methods on the same value, so pool waits are traced
// without separate wiring. Called after Setup.
func NewPgxTracer() *PgxTracer {
	return &PgxTracer{tracer: otel.Tracer("github.com/nhdewitt/spectra/internal/telemetry")}
}

func (t *PgxTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return t.start(ctx, queryName(data.SQL))
}

func (t *PgxTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	span := trace.SpanFromContext(ctx)
	// Rows written, or rows returned for a SELECT.
	if data.Err == nil && span.IsRecording() {
		span.SetAttributes(attribute.Int64("db.rows_affected", data.CommandTag.RowsAffected()))
	}
	EndSpan(span, data.Err)
}

func (t *PgxTracer) TraceAcquireStart(ctx context.Context, _ *pgxpool.Pool, _ pgxpool.TraceAcquireStartData) context.Context {
	return t.start(ctx, "pool.acquire")
}

func (t *PgxTracer) TraceAcquireEnd(ctx context.Context, _ *pgxpool.Pool, data pgxpool.TraceAcquireEndData) {
	EndSpan(trace.SpanFromContext(ctx), data.Err)
}

// start opens a span under the one in ctx, or returns ctx unchanged when there is none.
//
// pgx hands the context returned here to the matching End call, so End ends the span
// opened here. When start declined, ctx holds no valid span and End is a no-op, so there
// is no path where a query ends its parent.
func (t *PgxTracer) start(ctx context.Context, name string) context.Context {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	ctx, _ = t.tracer.Start(ctx, name,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(dbSystem),
	)
	return ctx
}

// queryName names a query span without allocating.
//
// sqlc begins every generated statement with "-- name: <Name> :<kind>", so the name is a
// substring of the SQL constant. Anything else is named by its first word.
func queryName(sql string) string {
	if rest, ok := strings.CutPrefix(sql, "-- name: "); ok {
		if i := strings.IndexAny(rest, " \n"); i > 0 {
			return rest[:i]
		}
	}

	sql = strings.TrimLeft(sql, " \t\r\n")
	if i := strings.IndexAny(sql, " \t\r\n;("); i > 0 {
		return sql[:i]
	}
	if sql == "" {
		return "query"
	}
	return sql
}
