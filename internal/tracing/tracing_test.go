package tracing

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestInit_DisabledIsNoop — when Enabled=false, Init must succeed and produce
// a working shutdown without registering any exporter.
func TestInit_DisabledIsNoop(t *testing.T) {
	shutdown, err := Init(context.Background(), Config{Enabled: false}, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err != nil {
		t.Fatalf("Init returned err with Enabled=false: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned err: %v", err)
	}
	// Producing a span should still work; it just goes nowhere.
	_, span := StartSpan(context.Background(), "noop")
	span.End()
}

// TestStartSpan_ProducesExportedSpan — with an in-memory exporter installed
// directly on the SDK, StartSpan should produce one finished span we can read.
func TestStartSpan_ProducesExportedSpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp),
	)
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(context.Background())

	_, span := StartSpan(context.Background(), "unit-test-span")
	span.End()

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	if spans[0].Name != "unit-test-span" {
		t.Errorf("span name = %q, want unit-test-span", spans[0].Name)
	}
}

// TestPropagator_InjectsTraceparent — once Init runs with the W3C propagator,
// Inject should populate traceparent into a header carrier.
func TestPropagator_InjectsTraceparent(t *testing.T) {
	if _, err := Init(context.Background(), Config{Enabled: false}, slog.New(slog.NewTextHandler(os.Stderr, nil))); err != nil {
		t.Fatal(err)
	}
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)

	ctx, span := tp.Tracer("test").Start(context.Background(), "outer")
	defer span.End()

	h := http.Header{}
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(h))
	if h.Get("traceparent") == "" {
		t.Fatal("traceparent header was not injected — propagator not registered?")
	}
}
