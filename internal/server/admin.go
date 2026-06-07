package server

import (
	"encoding/json"
	"net/http"
	"sync/atomic"

	"ai-gateway/internal/cache"
	"ai-gateway/internal/metrics"
)

type AdminHandler struct {
	server *Server
}

type AdminStats struct {
	CacheHits   atomic.Int64
	CacheMisses atomic.Int64
	TotalReqs   atomic.Int64
	TotalErrors atomic.Int64
	StreamReqs  atomic.Int64
}

var stats AdminStats

func recordReq() {
	stats.TotalReqs.Add(1)
}

func recordHit() {
	stats.CacheHits.Add(1)
	metrics.CacheHitsTotal.WithLabelValues("hit").Inc()
}
func recordMiss() {
	stats.CacheMisses.Add(1)
	metrics.CacheHitsTotal.WithLabelValues("miss").Inc()
}
func recordError() {
	stats.TotalErrors.Add(1)
}
func recordStream() {
	stats.StreamReqs.Add(1)
	metrics.StreamRequestsTotal.Inc()
}

func (s *Server) adminRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	ah := &AdminHandler{server: s}

	mux.HandleFunc("GET /admin/health", ah.handleHealth)
	mux.HandleFunc("GET /admin/routes", ah.handleRoutes)
	mux.HandleFunc("GET /admin/cache", ah.handleCache)

	return mux
}

func (ah *AdminHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

func (ah *AdminHandler) handleRoutes(w http.ResponseWriter, r *http.Request) {
	cfg := ah.server.config
	routes := make([]map[string]interface{}, 0, len(cfg.Routes))
	for _, rc := range cfg.Routes {
		route := map[string]interface{}{
			"name":     rc.Name,
			"match":    rc.Match.Model,
			"strategy": rc.Strategy,
			"targets":  rc.Targets,
		}
		if rc.Strategy == "semantic" {
			route["semantic_rules"] = rc.SemanticRules
		}
		routes = append(routes, route)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"routes": routes,
	})
}

func (ah *AdminHandler) handleCache(w http.ResponseWriter, r *http.Request) {
	total := stats.CacheHits.Load() + stats.CacheMisses.Load()
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(stats.CacheHits.Load()) / float64(total) * 100
	}

	var cacheSize int
	var entries []cache.CacheEntryInfo

	if mc, ok := ah.server.cacheBackend.(*cache.MemoryCache); ok {
		entries = mc.Info()
		cacheSize = len(entries)
	}

	info := map[string]interface{}{
		"cache_enabled":   ah.server.config.Cache.Enabled,
		"cache_backend":   ah.server.config.Cache.Backend,
		"cache_strategy":  ah.server.config.Cache.Strategy,
		"size":            cacheSize,
		"max_size":        ah.server.config.Cache.MaxSize,
		"hits":            stats.CacheHits.Load(),
		"misses":          stats.CacheMisses.Load(),
		"total_requests":  stats.TotalReqs.Load(),
		"total_errors":    stats.TotalErrors.Load(),
		"stream_requests": stats.StreamReqs.Load(),
		"hit_rate_pct":    hitRate,
		"entries":         entries,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}
