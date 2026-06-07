package provider

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// injectTraceContext writes the W3C traceparent (and tracestate, if any) for
// ctx into h. Called from every provider's buildRequest so the upstream LLM
// (or, more often, a reverse proxy in front of it) can include our trace.
//
// When tracing is disabled, the global propagator is propagation.TraceContext
// over a no-op SpanContext, which writes nothing to the headers — so this is
// cheap to call unconditionally.
func injectTraceContext(ctx context.Context, h http.Header) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(h))
}
