// Package telemetry wires OpenTelemetry tracing into the server.
//
// Tracing is opt-in. Setup installs nothing unless an OTLP endpoint is configured,
// and callers use the flag it returns to leave the HTTP middleware and the pgx
// tracer out of the request path entirely rather than running them against a no-op
// provider. Everything else comes from the standard OTEL_* environment variables.
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const serviceName = "spectra-server"

// Setup installs a global tracer provider exporting over OTLP/HTTP.
//
// It reports enabled=false and installs nothing when neither OTEL_EXPORTER_OTLP_ENDPOINT
// nor OTEL_EXPORTER_OTLP_TRACES_ENDPOINT is set. shutdown flushes buffered spans and is
// safe to call either way.
func Setup(ctx context.Context, serviceVersion string) (shutdown func(context.Context) error, enabled bool, err error) {
	noop := func(context.Context) error { return nil }

	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" && os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") == "" {
		return noop, false, nil
	}

	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return noop, false, fmt.Errorf("create OTLP trace exporter: %w", err)
	}

	// Env last to override the defaults
	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", serviceName),
			attribute.String("service.version", serviceVersion),
		),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithFromEnv(),
	)
	if err != nil && !errors.Is(err, resource.ErrPartialResource) {
		return noop, false, errors.Join(fmt.Errorf("build trace resource: %w", err), exporter.Shutdown(ctx))
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		slog.Warn("opentelemetry error", "error", err)
	}))

	return tp.Shutdown, true, nil
}

// EndSpan records err on span if non-nil and ends it.
func EndSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
