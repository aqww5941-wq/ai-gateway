package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"ai-gateway/config"
	"ai-gateway/internal/cache"
	"ai-gateway/internal/limiter"
	"ai-gateway/internal/middleware"
	"ai-gateway/internal/observer"
	"ai-gateway/internal/provider"
	"ai-gateway/internal/router"
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

	handler := withRecovery(logger, mainMux)
	handler = middleware.WithMetrics(logger, handler)

	if cfg.Auth.Enabled && len(cfg.Auth.Keys) > 0 {
		handler = middleware.Auth(cfg.Auth.Keys, logger, handler)
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
	var req provider.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

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

	target, err := s.router.Route(r.Context(), &req)
	if err != nil {
		s.logger.Warn("routing failed", "model", req.Model, "error", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	fallbackChain := s.router.FallbackChain(req.Model)
	obs := observer.New(target.Model, target.Provider)

	var resp *provider.ChatResponse
	if fallbackChain != nil {
		resp, err = s.tryWithFallback(r.Context(), &req, fallbackChain, obs)
	} else {
		req.Model = target.Model
		s.logger.Info("routing", "model", target.Model, "provider", target.Provider)
		prov := s.providers[target.Provider]
		resp, err = prov.ChatCompletion(r.Context(), &req)
	}

	if err != nil {
		recordError()
		s.logger.Error("upstream call failed", "error", err)
		obs.Finalize(s.logger, 0, 0, false, http.StatusBadGateway)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	// Store in cache
	if s.cacheBackend != nil && resp != nil {
		cacheKey := cache.CacheKey(&req)
		s.cacheBackend.Set(cacheKey, resp)
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

func (s *Server) tryWithFallback(ctx context.Context, req *provider.ChatRequest, chain []router.Target, obs *observer.Observer) (*provider.ChatResponse, error) {
	var lastErr error
	for i, t := range chain {
		prov, ok := s.providers[t.Provider]
		if !ok {
			lastErr = fmt.Errorf("provider %q not found", t.Provider)
			continue
		}
		req.Model = t.Model
		obs.Model = t.Model
		obs.Provider = t.Provider
		s.logger.Info("fallback attempt", "attempt", i+1, "provider", t.Provider, "model", t.Model)

		resp, err := prov.ChatCompletion(ctx, req)
		if err != nil {
			lastErr = err
			s.logger.Warn("fallback target failed", "provider", t.Provider, "error", err)
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("all fallback targets failed: %w", lastErr)
}

func (s *Server) handleStreamCompletion(w http.ResponseWriter, r *http.Request, req *provider.ChatRequest) {
	target, err := s.router.Route(r.Context(), req)
	if err != nil {
		s.logger.Warn("routing failed", "model", req.Model, "error", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	prov, ok := s.providers[target.Provider]
	if !ok {
		http.Error(w, "upstream provider not found", http.StatusInternalServerError)
		return
	}

	req.Model = target.Model
	s.logger.Info("stream routing", "model", target.Model, "provider", target.Provider)

	upstream, err := prov.ChatCompletionStream(r.Context(), req)
	if err != nil {
		s.logger.Error("upstream stream call failed", "error", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	obs := observer.New(target.Model, target.Provider)

	clientCh := make(chan *provider.StreamChunk, 16)
	collectorCh := make(chan *provider.StreamChunk, 16)

	var fullContent strings.Builder
	var promptTokens, completionTokens int

	// Broadcast upstream chunks to both consumers
	go func() {
		defer close(clientCh)
		defer close(collectorCh)
		for chunk := range upstream {
			clientCh <- chunk
			collectorCh <- chunk
		}
	}()

	// Write SSE to HTTP client
	go func() {
		flusher, ok := w.(http.Flusher)
		if !ok {
			s.logger.Error("streaming not supported by client")
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		for chunk := range clientCh {
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}()

	// Collect full response for caching + token counting
	for chunk := range collectorCh {
		for _, choice := range chunk.Choices {
			fullContent.WriteString(choice.Delta.Content)
		}
		if chunk.Usage != nil {
			promptTokens = chunk.Usage.PromptTokens
			completionTokens = chunk.Usage.CompletionTokens
		}
	}

	// Cache the complete response
	if s.cacheBackend != nil {
		cachedResp := &provider.ChatResponse{
			ID:      "cached",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   target.Model,
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
		cacheKey := cache.CacheKey(req)
		s.cacheBackend.Set(cacheKey, cachedResp)
		if s.semanticCache != nil {
			s.semanticCache.SetWithEmbedding(req, cachedResp)
		}
	}

	obs.Finalize(s.logger, promptTokens, completionTokens, false, http.StatusOK)
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
