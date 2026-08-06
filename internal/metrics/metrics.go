// Package metrics centralizes Prometheus collectors. All metrics share the
// "gateway_" namespace so they can be scraped under a single job.
//
// Label cardinality is the only thing that can blow Prometheus up at scale,
// so we pre-restrict label values: provider/model come from config (bounded),
// status is a small enum, and req_id stays in logs only.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	RequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Name:      "requests_total",
			Help:      "Total HTTP requests handled, by provider/model/status.",
		},
		[]string{"provider", "model", "status"},
	)

	RequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "gateway",
			Name:      "request_duration_seconds",
			Help:      "End-to-end request latency, by provider/model.",
			// Tuned for LLM workloads — buckets focus on 50 ms (cache hit)
			// through 30 s (worst-case upstream).
			Buckets: []float64{0.005, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		},
		[]string{"provider", "model"},
	)

	TokensTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Name:      "tokens_total",
			Help:      "Total tokens consumed, split by prompt/completion.",
		},
		[]string{"provider", "model", "kind"},
	)

	CostUSDTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Name:      "cost_usd_total",
			Help:      "Total USD spent on upstream calls, by model.",
		},
		[]string{"provider", "model"},
	)

	CacheHitsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Name:      "cache_hits_total",
			Help:      "Cache lookups, by result.",
		},
		[]string{"result"}, // hit | miss | semantic_hit
	)

	UpstreamErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Name:      "upstream_errors_total",
			Help:      "Upstream provider errors, by provider.",
		},
		[]string{"provider"},
	)

	InFlightRequests = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "gateway",
			Name:      "in_flight_requests",
			Help:      "Currently in-flight requests.",
		},
	)

	StreamRequestsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Name:      "stream_requests_total",
			Help:      "Streaming requests served.",
		},
	)

	// BreakerStateTransitions counts how often a circuit breaker changed state.
	// A spike on "open" tells SRE that an upstream is failing without needing
	// to read the application log.
	BreakerStateTransitions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Name:      "breaker_transitions_total",
			Help:      "Circuit breaker state transitions, by provider and new state.",
		},
		[]string{"provider", "state"}, // state: open | half_open | closed
	)

	// BreakerShortCircuitTotal counts requests that the breaker rejected
	// without hitting the upstream. High = the breaker is doing its job.
	BreakerShortCircuitTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Name:      "breaker_short_circuits_total",
			Help:      "Requests rejected by an open breaker, by provider.",
		},
		[]string{"provider"},
	)

	// RetryAttemptsTotal counts every retried attempt (excludes the initial try).
	// (#retries / #requests) is a useful upstream-stability signal.
	RetryAttemptsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Name:      "retry_attempts_total",
			Help:      "Retry attempts after the initial try, by provider.",
		},
		[]string{"provider"},
	)

	// CoalescedRequestsTotal counts requests that piggybacked on a singleflight
	// leader. Each one represents an upstream call we DID NOT make.
	CoalescedRequestsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Name:      "coalesced_requests_total",
			Help:      "Requests that shared a singleflight upstream call.",
		},
	)

	ConfigReloadTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Name:      "config_reload_total",
			Help:      "Runtime configuration reload attempts by result and stage.",
		},
		[]string{"result", "stage"}, // result: published | rejected | unchanged; stage: validation | restart_required | snapshot_build | compare | publish
	)
)

// Register installs all gateway collectors on the supplied registry. Returns
// an http.Handler that exposes /metrics in Prometheus text format.
func Register() http.Handler {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		RequestsTotal,
		RequestDuration,
		TokensTotal,
		CostUSDTotal,
		CacheHitsTotal,
		UpstreamErrorsTotal,
		InFlightRequests,
		StreamRequestsTotal,
		BreakerStateTransitions,
		BreakerShortCircuitTotal,
		RetryAttemptsTotal,
		CoalescedRequestsTotal,
		ConfigReloadTotal,
	)
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}

// Observe is the small helper handlers use to report a finished request.
// Centralizing this keeps the call sites readable.
func Observe(provider, model string, status int, latency time.Duration, promptTokens, completionTokens int, cost float64) {
	labels := prometheus.Labels{"provider": provider, "model": model, "status": strconv.Itoa(status)}
	RequestsTotal.With(labels).Inc()
	RequestDuration.WithLabelValues(provider, model).Observe(latency.Seconds())
	if promptTokens > 0 {
		TokensTotal.WithLabelValues(provider, model, "prompt").Add(float64(promptTokens))
	}
	if completionTokens > 0 {
		TokensTotal.WithLabelValues(provider, model, "completion").Add(float64(completionTokens))
	}
	if cost > 0 {
		CostUSDTotal.WithLabelValues(provider, model).Add(cost)
	}
}
