package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"ai-gateway/internal/cache"
	"ai-gateway/internal/router"
)

// GET /admin/api/v1/overview
func (s *Server) handleAdminOverview(w http.ResponseWriter, r *http.Request) {
	totalCalls, sharedCalls := s.coalescer.Stats()
	dedupRatio := 0.0
	if totalCalls > 0 {
		dedupRatio = float64(sharedCalls) / float64(totalCalls) * 100
	}

	totalReqs := stats.TotalReqs.Load()
	totalErrs := stats.TotalErrors.Load()
	hits := stats.CacheHits.Load()
	misses := stats.CacheMisses.Load()
	totalCacheChecks := hits + misses
	hitRate := 0.0
	if totalCacheChecks > 0 {
		hitRate = float64(hits) / float64(totalCacheChecks) * 100
	}
	errRate := 0.0
	if totalReqs > 0 {
		errRate = float64(totalErrs) / float64(totalReqs) * 100
	}

	writeJSON(w, map[string]interface{}{
		"uptime_seconds":     int(time.Since(startTime).Seconds()),
		"total_requests":     totalReqs,
		"cache_hits":         hits,
		"cache_misses":       misses,
		"hit_rate_pct":       hitRate,
		"total_errors":       totalErrs,
		"error_rate_pct":     errRate,
		"stream_requests":    stats.StreamReqs.Load(),
		"coalescer": map[string]interface{}{
			"total_calls":    totalCalls,
			"shared_calls":   sharedCalls,
			"dedup_ratio_pct": dedupRatio,
		},
		"cache_enabled":       s.config.Cache.Enabled,
		"cache_backend":       s.config.Cache.Backend,
		"cache_strategy":      s.config.Cache.Strategy,
		"rate_limit_enabled": s.config.RateLimit.Enabled,
	})
}

// GET /admin/api/v1/breakers
func (s *Server) handleAdminBreakers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]interface{}{
		"breakers": s.breakers.Snapshots(),
	})
}

// GET /admin/api/v1/providers
func (s *Server) handleAdminProviders(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	cfg := s.config
	providers := s.providers
	s.mu.RUnlock()

	type providerInfo struct {
		Name         string   `json:"name"`
		Type         string   `json:"type"`
		Models       []string `json:"models"`
		Health       string   `json:"health"`
		BreakerState string   `json:"breaker_state"`
		Timeout      string   `json:"timeout"`
	}

	out := make([]providerInfo, 0, len(cfg.Providers))
	for _, pc := range cfg.Providers {
		p, ok := providers[pc.Name]
		if !ok {
			continue
		}
		br := s.breakers.Get(pc.Name)
		state := br.State().String()
		health := "healthy"
		switch state {
		case "open":
			health = "unhealthy"
		case "half-open":
			health = "degraded"
		}
		timeout := pc.Timeout
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		out = append(out, providerInfo{
			Name:         p.Name(),
			Type:         pc.Type,
			Models:       p.SupportedModels(),
			Health:       health,
			BreakerState: state,
			Timeout:      timeout.String(),
		})
	}
	writeJSON(w, map[string]interface{}{
		"providers": out,
	})
}

// GET /admin/api/v1/latency
func (s *Server) handleAdminLatency(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	cfg := s.config
	s.mu.RUnlock()
	snapshots := s.router.LatencySnapshots(cfg.Routes)
	if snapshots == nil {
		snapshots = []router.RouteLatencySnapshot{}
	}
	writeJSON(w, map[string]interface{}{
		"routes": snapshots,
	})
}

// GET /admin/api/v1/routes
func (s *Server) handleAdminRoutesV1(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	cfg := s.config
	s.mu.RUnlock()

	type targetInfo struct {
		Provider string `json:"provider"`
		Model    string `json:"model"`
		Weight   int    `json:"weight"`
	}
	type semanticRule struct {
		Complexity string `json:"complexity"`
		Provider   string `json:"provider"`
		Model      string `json:"model"`
	}
	type routeInfo struct {
		Name          string          `json:"name"`
		MatchModel    string          `json:"match_model"`
		Strategy      string          `json:"strategy"`
		Targets       []targetInfo    `json:"targets"`
		SemanticRules []semanticRule  `json:"semantic_rules"`
	}

	routes := make([]routeInfo, 0, len(cfg.Routes))
	for _, rc := range cfg.Routes {
		ri := routeInfo{
			Name:       rc.Name,
			MatchModel: rc.Match.Model,
			Strategy:   rc.Strategy,
			Targets:    make([]targetInfo, 0, len(rc.Targets)),
		}
		for _, t := range rc.Targets {
			ri.Targets = append(ri.Targets, targetInfo{
				Provider: t.Provider,
				Model:    t.Model,
				Weight:   t.Weight,
			})
		}
		if rc.Strategy == "semantic" {
			ri.SemanticRules = make([]semanticRule, 0, len(rc.SemanticRules))
			for _, sr := range rc.SemanticRules {
				ri.SemanticRules = append(ri.SemanticRules, semanticRule{
					Complexity: sr.Complexity,
					Provider:   sr.Target.Provider,
					Model:      sr.Target.Model,
				})
			}
		}
		routes = append(routes, ri)
	}
	writeJSON(w, map[string]interface{}{
		"routes": routes,
	})
}

// GET /admin/api/v1/cache
func (s *Server) handleAdminCacheV1(w http.ResponseWriter, r *http.Request) {
	hits := stats.CacheHits.Load()
	misses := stats.CacheMisses.Load()
	totalCache := hits + misses
	hitRate := 0.0
	if totalCache > 0 {
		hitRate = float64(hits) / float64(totalCache) * 100
	}

	var cacheSize int
	var entries []cache.CacheEntryInfo
	if mc, ok := s.cacheBackend.(*cache.MemoryCache); ok {
		entries = mc.Info()
		cacheSize = mc.Len()
	}

	writeJSON(w, map[string]interface{}{
		"enabled":       s.config.Cache.Enabled,
		"backend":       s.config.Cache.Backend,
		"strategy":      s.config.Cache.Strategy,
		"max_size":      s.config.Cache.MaxSize,
		"current_size":  cacheSize,
		"hits":          hits,
		"misses":        misses,
		"hit_rate_pct":  hitRate,
		"entries":       entries,
	})
}

// GET /admin/api/v1/cache/entries/{key}
func (s *Server) handleAdminCacheEntry(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/admin/api/v1/cache/entries/")
	if key == "" {
		http.Error(w, "missing key", http.StatusBadRequest)
		return
	}

	mc, ok := s.cacheBackend.(*cache.MemoryCache)
	if !ok {
		http.Error(w, "entry lookup only supported for memory cache", http.StatusNotImplemented)
		return
	}

	entry, found := mc.Entry(key)
	if !found {
		http.Error(w, "entry not found or expired", http.StatusNotFound)
		return
	}

	writeJSON(w, entry)
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}