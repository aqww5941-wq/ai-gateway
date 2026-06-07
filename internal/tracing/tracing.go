// Package tracing wraps OpenTelemetry's SDK so the rest of the gateway can
// add spans without depending on the SDK directly.
//
// Two exporters are supported:
//
//	"otlp"   — push spans to an OTLP/HTTP collector (e.g. Jaeger, Tempo,
//	          Honeycomb). The endpoint comes from OTEL_EXPORTER_OTLP_ENDPOINT.
//	"stdout" — write JSON spans to stderr. Useful for local dev / demos.
//
// When tracing is disabled (the default) we install a no-op tracer provider
// so every call to StartSpan still works but allocates nothing of consequence.
// This keeps the call sites unchanged whether tracing is on or off.
package tracing

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Config controls how tracing is initialized.
type Config struct {
	// Enabled gates the entire subsystem. When false, Init returns a no-op
	// shutdown function and leaves the global TracerProvider as the SDK's
	// default no-op implementation.
	Enabled bool

	// Exporter picks the backend ("otlp" or "stdout"). Ignored when Enabled
	// is false.
	Exporter string

	// ServiceName is reported as service.name on every span. Defaults to
	// "ai-gateway".
	ServiceName string

	// SampleRatio: 0..1. 1.0 = sample every span, 0.1 = ~10%. Defaults to 1.0
	// because the gateway's per-request volume is modest and SREs typically
	// want full traces during incident debugging.
	SampleRatio float64
}

// Init wires up OpenTelemetry. Callers should defer the returned shutdown to
// flush any in-flight spans before the process exits.
//
// If Enabled is false, returns a no-op shutdown immediately — safe to call
// in tests or in deployments without a collector.
func Init(ctx context.Context, cfg Config, logger *slog.Logger) (func(context.Context) error, error) {
	if !cfg.Enabled {
		// Even when disabled, install a W3C propagator so we honor incoming
		// trace context headers (so the gateway shows up as a span in the
		// caller's trace even if we don't export our own).
		otel.SetTextMapPropagator(propagation.TraceContext{})
		return func(context.Context) error { return nil }, nil
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = "ai-gateway"
	}
	if cfg.SampleRatio <= 0 || cfg.SampleRatio > 1 {
		cfg.SampleRatio = 1.0
	}

	exp, err := buildExporter(ctx, cfg.Exporter)
	if err != nil {
		return nil, fmt.Errorf("build exporter: %w", err)
	}

	res, err := sdkresource.New(ctx,
		sdkresource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("build resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.SampleRatio)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	logger.Info("tracing enabled", "exporter", cfg.Exporter, "service", cfg.ServiceName, "sample_ratio", cfg.SampleRatio)
	return tp.Shutdown, nil
}

func buildExporter(ctx context.Context, name string) (sdktrace.SpanExporter, error) {
	switch name {
	case "stdout", "":
		return stdouttrace.New(stdouttrace.WithPrettyPrint())
	case "otlp":
		// Endpoint comes from OTEL_EXPORTER_OTLP_ENDPOINT (or the package default).
		return otlptracehttp.New(ctx)
	default:
		return nil, fmt.Errorf("unknown tracing exporter %q (want stdout|otlp)", name)
	}
}

// Tracer returns the gateway's named tracer. All call sites should use this
// rather than otel.Tracer directly so we can swap implementations in tests.
func Tracer() trace.Tracer {
	return otel.Tracer("ai-gateway")
}

// StartSpan is a thin wrapper to keep call sites short. The returned context
// MUST be propagated downstream so child spans link correctly.
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name, opts...)
}

// PropagatorHTTPHeaders is a helper for the provider HTTP client: it returns
// the headers that should be added to outgoing requests so the upstream sees
// our trace context. We separate this from the global TextMapPropagator
// because the provider package shouldn't import otel directly.
//
// Returns nil when tracing is disabled.
func PropagatorHTTPHeaders(ctx context.Context) map[string]string {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	if len(carrier) == 0 {
		return nil
	}
	return carrier
}
