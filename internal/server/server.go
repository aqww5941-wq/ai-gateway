package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"ai-gateway/config"
	"ai-gateway/internal/breaker"
	"ai-gateway/internal/cache"
	"ai-gateway/internal/limiter"
	"ai-gateway/internal/metrics"
	"ai-gateway/internal/middleware"
	"ai-gateway/internal/observer"
	"ai-gateway/internal/provider"
	"ai-gateway/internal/retry"
	"ai-gateway/internal/router"
	"ai-gateway/internal/tracing"
)

type Server struct {
	config        *config.Config
	router        *router.Router
	providers     map[string]provider.LLMProvider
	mu            sync.RWMutex
	logger        *slog.Logger
	httpSrv       *http.Server
	cacheBackend  cache.CacheBackend
	semanticCache *cache.SemanticCache
	keyLimiter    *limiter.TokenBucketLimiter
	modelLimiter  *limiter.TokenBucketLimiter

	// Resilience layer — one breaker per provider, one coalescer for the
	// whole gateway, one shared retry policy. All three are nil-safe to
	// disable, but they're always on in production.
	breakers   *breaker.Manager
	coalescer  *cache.Coalescer
	retryPolicy retry.Policy
}

func New(cfg *config.Config, r *router.Router, provs map[string]provider.LLMProvider, logger *slog.Logger) (*Server, error) {
	if len(provs) == 0 {
		return nil, fmt.Errorf("no providers configured")
	}
	s := &Server{
		config:    cfg,
		router:    r,
		providers: provs,
		logger:    logger,
		// Defaults are tuned for an LLM gateway: 5 consecutive failures
		// before tripping (gives ~10 s of badness budget at typical QPS),
		// 10 s cool-down (long enough for transient upstream noise to clear),
		// 2 consecutive probe successes to close (avoids flaps).
		breakers:  breaker.NewManager(breaker.Config{}),
		coalescer: cache.NewCoalescer(),
		retryPolicy: retry.Policy{
			MaxAttempts: 3,
			BaseDelay:   200 * time.Millisecond,
			MaxDelay:    2 * time.Second,
		},
	}

	// Setup cache
	if cfg.Cache.Enabled {
		ttl, err := time.ParseDuration(cfg.Cache.TTL)
		if err != nil {
			ttl = 1 * time.Hour
		}
		maxSize := cfg.Cache.MaxSize
		if maxSize <= 0 {
			maxSize = 1000
		}

		switch cfg.Cache.Backend {
		case "redis":
			redisCache, err := cache.NewRedisCache(cfg.Cache.RedisAddr, cfg.Cache.RedisPass, cfg.Cache.RedisDB, ttl)
			if err != nil {
				logger.Warn("redis cache unavailable, falling back to memory", "error", err)
				s.cacheBackend = cache.NewMemoryCache(maxSize, ttl)
			} else {
				s.cacheBackend = redisCache
				logger.Info("redis cache connected", "addr", cfg.Cache.RedisAddr)
			}
		default:
			memCache := cache.NewMemoryCache(maxSize, ttl)
			if cfg.Cache.Strategy == "semantic" {
				threshold := cfg.Cache.Threshold
				if threshold <= 0 {
					threshold = 0.85
				}
				s.semanticCache = cache.NewSemanticCache(maxSize, ttl, threshold)
				s.cacheBackend = s.semanticCache.MemoryCache
				logger.Info("semantic cache enabled", "threshold", threshold)
			} else {
				s.cacheBackend = memCache
			}
		}
	}

	// Setup rate limiters
	if cfg.RateLimit.Enabled {
		if cfg.RateLimit.PerKey > 0 {
			s.keyLimiter = limiter.NewTokenBucketLimiter(cfg.RateLimit.PerKey)
		}
		if cfg.RateLimit.PerModel > 0 {
			s.modelLimiter = limiter.NewTokenBucketLimiter(cfg.RateLimit.PerModel)
		}
	}

	adminMux := s.adminRoutes()
	mainMux := http.NewServeMux()
	mainMux.HandleFunc("POST /v1/chat/completions", s.handleChatCompletion)
	// Route /admin/ to adminMux
	mainMux.Handle("/admin/", adminMux)
	// Expose Prometheus scrape endpoint.
	mainMux.Handle("GET /metrics", metrics.Register())

	handler := withRecovery(logger, mainMux)
	handler = middleware.WithMetrics(logger, handler)
	handler = inFlightInstrumented(handler)

	if cfg.Auth.Enabled && len(cfg.Auth.Keys) > 0 {
		handler = middleware.NewAuth(cfg.Auth.Keys).Wrap(logger, handler)
	}

	if cfg.RateLimit.Enabled {
		handler = middleware.RateLimit(s.keyLimiter, s.modelLimiter, logger, handler)
	}

	s.httpSrv = &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
	}
	return s, nil
}

func (s *Server) Start() error {
	s.logger.Info("gateway starting", "port", s.config.Server.Port)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("server error", "error", err)
		}
	}()

	<-ctx.Done()
	s.logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.httpSrv.Shutdown(shutdownCtx)
}

// Reload hot-swaps the router and providers from a new config without downtime.
func (s *Server) Reload(cfg *config.Config) error {
	// Build new providers
	newProvs := make(map[string]provider.LLMProvider, len(cfg.Providers))
	for _, pCfg := range cfg.Providers {
		pLogger := s.logger.With("provider", pCfg.Name)
		switch pCfg.Type {
		case "openai":
			p, err := provider.NewOpenAI(pCfg, pLogger)
			if err != nil {
				return fmt.Errorf("provider %q: %w", pCfg.Name, err)
			}
			newProvs[pCfg.Name] = p
		case "claude":
			p, err := provider.NewClaude(pCfg, pLogger)
			if err != nil {
				return fmt.Errorf("provider %q: %w", pCfg.Name, err)
			}
			newProvs[pCfg.Name] = p
		default:
			return fmt.Errorf("unknown provider type %q for %q", pCfg.Type, pCfg.Name)
		}
	}

	// Build new router
	newRouter, err := router.NewRouter(cfg.Routes)
	if err != nil {
		return fmt.Errorf("router: %w", err)
	}

	// Validate targets reference known providers
	for _, routeCfg := range cfg.Routes {
		for _, t := range routeCfg.Targets {
			if _, ok := newProvs[t.Provider]; !ok {
				return fmt.Errorf("route %q references unknown provider %q", routeCfg.Name, t.Provider)
			}
		}
		for _, sr := range routeCfg.SemanticRules {
			if _, ok := newProvs[sr.Target.Provider]; !ok {
				return fmt.Errorf("route %q references unknown provider %q", routeCfg.Name, sr.Target.Provider)
			}
		}
	}

	// Atomic swap
	s.mu.Lock()
	s.config = cfg
	s.router = newRouter
	s.providers = newProvs
	s.mu.Unlock()

	s.logger.Info("config hot-reloaded successfully")
	return nil
}

func (s *Server) handleChatCompletion(w http.ResponseWriter, r *http.Request) {
	// gateway.handle is the root span for the request. We propagate
	// W3C traceparent from the incoming request so this span links to the
	// caller's trace (e.g. an upstream service that already started one).
	ctx, span := tracing.StartSpan(r.Context(), "gateway.handle")
	defer span.End()
	r = r.WithContext(ctx)

	var req provider.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	span.SetAttributes(otelAttrString("llm.model", req.Model), otelAttrBool("llm.stream", req.Stream))

	recordReq()

	if req.Stream {
		recordStream()
		s.handleStreamCompletion(w, r, &req)
		return
	}

	// Check cache
	if s.cacheBackend != nil {
		key := cache.CacheKey(&req)
		if resp, hit := s.cacheBackend.Get(key); hit {
			recordHit()
			s.logger.Info("cache hit", "model", req.Model)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		recordMiss()
	}

	// Check semantic cache
	if s.semanticCache != nil {
		if resp, hit := s.semanticCache.GetSimilar(&req); hit {
			recordHit()
			s.logger.Info("semantic cache hit", "model", req.Model)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
	}

	routeCtx, routeSpan := tracing.StartSpan(r.Context(), "route.select")
	target, err := s.router.Route(routeCtx, &req)
	if target != nil {
		routeSpan.SetAttributes(otelAttrString("route.provider", target.Provider), otelAttrString("route.model", target.Model))
	}
	routeSpan.End()
	if err != nil {
		s.logger.Warn("routing failed", "model", req.Model, "error", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// originalModel is what the client asked for; it's used to look up the
	// route's latency strategy (req.Model gets mutated to the upstream model
	// before the provider call).
	originalModel := req.Model
	fallbackChain := s.router.FallbackChain(req.Model)
	obs := observer.New(target.Model, target.Provider)

	// Singleflight on the cache key — concurrent identical cache-miss requests
	// share a single upstream call instead of fanning out.
	sfKey := cache.CacheKey(&req)
	resp, shared, err := s.coalescer.Do(r.Context(), sfKey, func(ctx context.Context) (*provider.ChatResponse, error) {
		if fallbackChain != nil {
			return s.tryWithFallback(ctx, &req, originalModel, fallbackChain, obs)
		}
		req.Model = target.Model
		s.logger.Info("routing", "model", target.Model, "provider", target.Provider)
		return s.callProviderUnary(ctx, target.Provider, target.Model, originalModel, &req)
	})
	if shared {
		metrics.CoalescedRequestsTotal.Inc()
	}

	if err != nil {
		recordError()
		metrics.UpstreamErrorsTotal.WithLabelValues(target.Provider).Inc()
		s.logger.Error("upstream call failed", "error", err)
		obs.Finalize(s.logger, 0, 0, false, http.StatusBadGateway)
		http.Error(w, err.Error(), s.statusFor(err))
		return
	}

	// Store in cache — use sfKey (computed before routing mutated req.Model)
	// so lookup and storage always use the same key.
	if s.cacheBackend != nil && resp != nil {
		s.cacheBackend.Set(sfKey, resp)
		if s.semanticCache != nil {
			s.semanticCache.SetWithEmbedding(&req, resp)
		}
	}

	obs.Finalize(s.logger, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, false, http.StatusOK)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.logger.Error("write response failed", "error", err)
	}
}

func (s *Server) tryWithFallback(ctx context.Context, req *provider.ChatRequest, originalModel string, chain []router.Target, obs *observer.Observer) (*provider.ChatResponse, error) {
	var lastErr error
	for i, t := range chain {
		if _, ok := s.providers[t.Provider]; !ok {
			lastErr = fmt.Errorf("provider %q not found", t.Provider)
			continue
		}
		req.Model = t.Model
		obs.Model = t.Model
		obs.Provider = t.Provider
		s.logger.Info("fallback attempt", "attempt", i+1, "provider", t.Provider, "model", t.Model)

		resp, err := s.callProviderUnary(ctx, t.Provider, t.Model, originalModel, req)
		if err != nil {
			lastErr = err
			s.logger.Warn("fallback target failed", "provider", t.Provider, "error", err)
			// Honor ctx cancellation — no point walking the rest of the chain.
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("all fallback targets failed: %w", lastErr)
}

// callProviderUnary issues a single non-streaming ChatCompletion, guarded by
// the provider's circuit breaker and wrapped in the retry policy.
//
// Layering:
//
//	breaker.Allow → retry.Do(provider.ChatCompletion → record outcome) → breaker.On{Success,Failure}
//
// Why retry runs *inside* the breaker (rather than the other way around):
// each retry is conceptually one upstream attempt for the breaker's accounting,
// so all attempts count as a single "call" — otherwise the retry storm would
// pop the breaker open instantly on the very first user request.
//
// We also time the call and feed it into the latency strategy (if any) so
// future routing decisions can favor faster providers.
func (s *Server) callProviderUnary(ctx context.Context, providerName, modelName, originalModel string, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	ctx, span := tracing.StartSpan(ctx, "provider.call")
	span.SetAttributes(otelAttrString("provider.name", providerName), otelAttrString("llm.model", modelName))
	defer span.End()

	br := s.breakers.Get(providerName)
	if err := br.Allow(); err != nil {
		span.SetAttributes(otelAttrBool("breaker.short_circuit", true))
		metrics.BreakerShortCircuitTotal.WithLabelValues(providerName).Inc()
		s.logger.Warn("breaker open, skipping provider", "provider", providerName)
		return nil, err
	}

	prov, ok := s.providers[providerName]
	if !ok {
		br.OnFailure()
		return nil, fmt.Errorf("provider %q not found", providerName)
	}

	start := time.Now()
	var attempts int
	resp, err := retry.Do(ctx, s.retryPolicy, func(ctx context.Context, attempt int) (*provider.ChatResponse, error) {
		attempts = attempt
		if attempt > 1 {
			metrics.RetryAttemptsTotal.WithLabelValues(providerName).Inc()
			s.logger.Info("retrying upstream call", "provider", providerName, "attempt", attempt)
		}
		return prov.ChatCompletion(ctx, req)
	})
	span.SetAttributes(otelAttrInt("provider.attempts", attempts))
	if err != nil {
		span.RecordError(err)
	} else if resp != nil {
		span.SetAttributes(
			otelAttrInt("llm.prompt_tokens", resp.Usage.PromptTokens),
			otelAttrInt("llm.completion_tokens", resp.Usage.CompletionTokens),
		)
	}
	s.recordBreakerOutcome(br, providerName, err)
	s.recordLatencySample(originalModel, providerName, modelName, time.Since(start), err)
	return resp, err
}

// recordLatencySample feeds an observation into the latency strategy attached
// to the request's route, if any. Routes that use other strategies get a no-op.
//
// originModel is the model the *client* asked for (used to look up the route);
// providerName/modelName are what we actually called (the strategy keys on the
// target tuple, not the client request).
func (s *Server) recordLatencySample(originModel, providerName, modelName string, d time.Duration, err error) {
	ls := s.router.LatencyStrategyFor(originModel)
	if ls == nil {
		return
	}
	failed := false
	if err != nil {
		if ue := provider.AsUpstream(err); ue != nil {
			failed = ue.IsRetryable()
		} else if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			failed = true
		}
	}
	ls.Observe(router.Target{Provider: providerName, Model: modelName}, d, failed)
}

// recordBreakerOutcome translates a call result into a breaker transition and
// emits a metric whenever the state actually changes.
//
// Client-side errors (4xx other than 429) and ctx cancellations should NOT
// trip the breaker — those say "bad request from the user", not "upstream is
// broken". So we treat them as neutral (no success, no failure).
func (s *Server) recordBreakerOutcome(br *breaker.Breaker, providerName string, err error) {
	prev := br.State()
	defer func() {
		if cur := br.State(); cur != prev {
			metrics.BreakerStateTransitions.WithLabelValues(providerName, cur.String()).Inc()
			s.logger.Warn("breaker state changed", "provider", providerName, "from", prev.String(), "to", cur.String())
		}
	}()

	if err == nil {
		br.OnSuccess()
		return
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}
	if ue := provider.AsUpstream(err); ue != nil {
		// Only 5xx / 429 / 408 are "upstream's fault" — 4xx are the client's.
		if !ue.IsRetryable() {
			return
		}
	}
	br.OnFailure()
}

// statusFor maps an upstream error back to an HTTP status code the client
// sees. The default is 502 (we couldn't reach the upstream).
func (s *Server) statusFor(err error) int {
	if errors.Is(err, breaker.ErrOpen) {
		return http.StatusServiceUnavailable // 503 — try later
	}
	if ue := provider.AsUpstream(err); ue != nil {
		// Bubble through certain upstream statuses; client should see 429 as 429.
		if ue.StatusCode == http.StatusTooManyRequests {
			return http.StatusTooManyRequests
		}
	}
	return http.StatusBadGateway
}

// handleStreamCompletion serves an SSE stream. The design goals are:
//
//  1. Cache parity with non-streaming: a cached response is replayed as an
//     SSE event sequence so streaming clients also benefit from caching.
//  2. Fan-out without head-of-line blocking: a single producer pumps upstream
//     chunks into two channels, both writes guarded by ctx so a stalled client
//     cannot pin the goroutine indefinitely.
//  3. http.ResponseWriter is owned by exactly one goroutine (the handler) — the
//     original implementation had a write race where the SSE-writer goroutine
//     could still be flushing after the handler returned.
//  4. Fallback parity with non-streaming: if the upstream call itself fails
//     before any chunk is sent we transparently try the next provider.
func (s *Server) handleStreamCompletion(w http.ResponseWriter, r *http.Request, req *provider.ChatRequest) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	ctx, span := tracing.StartSpan(ctx, "gateway.handle.stream")
	span.SetAttributes(otelAttrString("llm.model", req.Model))
	defer span.End()

	// 1. Cache replay — exact match first, then semantic.
	// Save original model for cache key — routing will mutate req.Model.
	origCacheKey := cache.CacheKey(req)
	if s.cacheBackend != nil {
		if resp, hit := s.cacheBackend.Get(origCacheKey); hit {
			recordHit()
			s.logger.Info("stream cache hit", "model", req.Model)
			s.replayCachedAsStream(w, resp)
			return
		}
		recordMiss()
	}
	if s.semanticCache != nil {
		if resp, hit := s.semanticCache.GetSimilar(req); hit {
			recordHit()
			s.logger.Info("stream semantic cache hit", "model", req.Model)
			s.replayCachedAsStream(w, resp)
			return
		}
	}

	// 2. Route + open stream (with fallback if configured).
	routeCtx, routeSpan := tracing.StartSpan(ctx, "route.select")
	target, err := s.router.Route(routeCtx, req)
	if target != nil {
		routeSpan.SetAttributes(otelAttrString("route.provider", target.Provider), otelAttrString("route.model", target.Model))
	}
	routeSpan.End()
	if err != nil {
		s.logger.Warn("routing failed", "model", req.Model, "error", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	fallbackChain := s.router.FallbackChain(req.Model)
	originalModel := req.Model
	var upstream <-chan *provider.StreamChunk
	var usedTarget router.Target

	if fallbackChain != nil {
		upstream, usedTarget, err = s.openStreamWithFallback(ctx, req, originalModel, fallbackChain)
	} else {
		req.Model = target.Model
		usedTarget = *target
		prov, ok := s.providers[target.Provider]
		if !ok {
			http.Error(w, "upstream provider not found", http.StatusInternalServerError)
			return
		}
		// Breaker-guard the single-target stream open. No retry for streams.
		br := s.breakers.Get(target.Provider)
		if berr := br.Allow(); berr != nil {
			metrics.BreakerShortCircuitTotal.WithLabelValues(target.Provider).Inc()
			s.logger.Warn("stream rejected: breaker open", "provider", target.Provider)
			http.Error(w, berr.Error(), http.StatusServiceUnavailable)
			return
		}
		start := time.Now()
		upstream, err = prov.ChatCompletionStream(ctx, req)
		s.recordBreakerOutcome(br, target.Provider, err)
		s.recordLatencySample(originalModel, target.Provider, target.Model, time.Since(start), err)
	}

	if err != nil {
		recordError()
		metrics.UpstreamErrorsTotal.WithLabelValues(usedTarget.Provider).Inc()
		s.logger.Error("upstream stream call failed", "error", err)
		http.Error(w, err.Error(), s.statusFor(err))
		return
	}

	obs := observer.New(usedTarget.Model, usedTarget.Provider)
	s.logger.Info("stream routing", "model", usedTarget.Model, "provider", usedTarget.Provider)

	// 3. Set SSE headers BEFORE any goroutine spawns, so there's no race
	//    between WriteHeader and any flusher.Flush.
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	// 4. Fan-out. Buffers sized to absorb short bursts; the producer respects
	//    ctx cancellation on both sends so we never leak.
	const fanBuf = 128
	clientCh := make(chan *provider.StreamChunk, fanBuf)
	collectorCh := make(chan *provider.StreamChunk, fanBuf)

	go func() {
		defer close(clientCh)
		defer close(collectorCh)
		for chunk := range upstream {
			select {
			case clientCh <- chunk:
			case <-ctx.Done():
				return
			}
			select {
			case collectorCh <- chunk:
			case <-ctx.Done():
				return
			}
		}
	}()

	// 5. Collector runs concurrently with the client writer so collection
	//    speed never blocks the SSE flush path.
	var fullContent strings.Builder
	var promptTokens, completionTokens int
	collectorDone := make(chan struct{})
	go func() {
		defer close(collectorDone)
		for chunk := range collectorCh {
			for _, choice := range chunk.Choices {
				fullContent.WriteString(choice.Delta.Content)
			}
			if chunk.Usage != nil {
				promptTokens = chunk.Usage.PromptTokens
				completionTokens = chunk.Usage.CompletionTokens
			}
		}
	}()

	// 6. The handler goroutine owns http.ResponseWriter and is the only
	//    place that calls Write/Flush — no data race possible.
	for chunk := range clientCh {
		data, _ := json.Marshal(chunk)
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			// Client gone — cancel ctx so producer/collector exit promptly.
			s.logger.Warn("client write failed, aborting stream", "error", err)
			cancel()
			break
		}
		flusher.Flush()
	}
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()

	// 7. Wait for collector before writing cache to avoid racing with
	//    in-flight token-count updates.
	<-collectorDone

	// 8. Cache the fully assembled response.
	// Use origCacheKey (computed before routing mutated req.Model) so
	// lookup and storage always use the same key.
	if s.cacheBackend != nil && fullContent.Len() > 0 {
		cachedResp := &provider.ChatResponse{
			ID:      "cached",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   usedTarget.Model,
			Choices: []provider.Choice{
				{
					Index: 0,
					Message: provider.Message{
						Role:    "assistant",
						Content: fullContent.String(),
					},
					FinishReason: "stop",
				},
			},
			Usage: provider.Usage{
				PromptTokens:     promptTokens,
				CompletionTokens: completionTokens,
				TotalTokens:      promptTokens + completionTokens,
			},
		}
		s.cacheBackend.Set(origCacheKey, cachedResp)
		if s.semanticCache != nil {
			s.semanticCache.SetWithEmbedding(req, cachedResp)
		}
	}

	obs.Finalize(s.logger, promptTokens, completionTokens, false, http.StatusOK)
}

// openStreamWithFallback walks the fallback chain trying each provider until
// one successfully opens a stream. We can only retry before any chunk is
// produced — once the client has seen "data:" bytes we cannot rewind.
//
// Per-target is wrapped in its circuit breaker. We do NOT use the retry
// helper here: a stream open that fails to even establish the connection is
// instead retried implicitly via the next entry in the fallback chain, which
// gives the same effect plus the benefit of trying a different provider.
func (s *Server) openStreamWithFallback(ctx context.Context, req *provider.ChatRequest, originalModel string, chain []router.Target) (<-chan *provider.StreamChunk, router.Target, error) {
	var lastErr error
	for i, t := range chain {
		prov, ok := s.providers[t.Provider]
		if !ok {
			lastErr = fmt.Errorf("provider %q not found", t.Provider)
			continue
		}
		br := s.breakers.Get(t.Provider)
		if err := br.Allow(); err != nil {
			metrics.BreakerShortCircuitTotal.WithLabelValues(t.Provider).Inc()
			s.logger.Warn("stream skipped: breaker open", "provider", t.Provider)
			lastErr = err
			continue
		}
		req.Model = t.Model
		s.logger.Info("stream fallback attempt", "attempt", i+1, "provider", t.Provider, "model", t.Model)
		start := time.Now()
		upstream, err := prov.ChatCompletionStream(ctx, req)
		s.recordBreakerOutcome(br, t.Provider, err)
		s.recordLatencySample(originalModel, t.Provider, t.Model, time.Since(start), err)
		if err != nil {
			lastErr = err
			s.logger.Warn("stream fallback target failed", "provider", t.Provider, "error", err)
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, router.Target{}, err
			}
			continue
		}
		return upstream, t, nil
	}
	return nil, router.Target{}, fmt.Errorf("all stream fallback targets failed: %w", lastErr)
}

// replayCachedAsStream serves a cached non-streaming response as an SSE event
// sequence so streaming clients benefit from caching. We split the content
// into modest-sized chunks to preserve perceived-progressive UX rather than
// dumping the whole answer in one event.
func (s *Server) replayCachedAsStream(w http.ResponseWriter, resp *provider.ChatResponse) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Cache", "HIT")
	w.WriteHeader(http.StatusOK)

	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	const chunkSize = 64
	runes := []rune(content)
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunk := provider.StreamChunk{
			ID:      resp.ID,
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   resp.Model,
			Choices: []provider.StreamChoice{
				{
					Index: 0,
					Delta: provider.StreamDelta{Content: string(runes[i:end])},
				},
			},
		}
		data, _ := json.Marshal(&chunk)
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return
		}
		flusher.Flush()
	}
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func withRecovery(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("panic recovered", "panic", rec)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// inFlightInstrumented increments the in-flight gauge for the duration of
// each request. /metrics scrapes are excluded so Prometheus polling doesn't
// pollute the gauge.
func inFlightInstrumented(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		metrics.InFlightRequests.Inc()
		defer metrics.InFlightRequests.Dec()
		next.ServeHTTP(w, r)
	})
}
