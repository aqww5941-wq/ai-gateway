package observer

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"

	"ai-gateway/internal/metrics"
)

type Observer struct {
	ReqID            string
	StartTime        time.Time
	Model            string
	Provider         string
	PromptTokens     int
	CompletionTokens int
	Cost             float64
	Latency          time.Duration
	CacheHit         bool
	Status           int
}

func New(model, provider string) *Observer {
	return &Observer{
		ReqID:     generateID(),
		StartTime: time.Now(),
		Model:     model,
		Provider:  provider,
	}
}

func (o *Observer) Finalize(logger *slog.Logger, promptTokens, completionTokens int, cacheHit bool, status int) {
	o.PromptTokens = promptTokens
	o.CompletionTokens = completionTokens
	o.Latency = time.Since(o.StartTime)
	o.CacheHit = cacheHit
	o.Status = status
	o.Cost = CalculateCost(o.Model, promptTokens, completionTokens)

	logger.Info("request completed",
		"req_id", o.ReqID,
		"model", o.Model,
		"provider", o.Provider,
		"tokens", map[string]int{
			"prompt":     o.PromptTokens,
			"completion": o.CompletionTokens,
		},
		"cost", o.Cost,
		"latency_ms", o.Latency.Milliseconds(),
		"cache_hit", o.CacheHit,
		"status", o.Status,
	)

	// Export to Prometheus. Cache hits are reported with provider="cache"
	// so dashboards can distinguish them from upstream traffic.
	provider := o.Provider
	if cacheHit {
		provider = "cache"
	}
	metrics.Observe(provider, o.Model, status, o.Latency, promptTokens, completionTokens, o.Cost)
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
