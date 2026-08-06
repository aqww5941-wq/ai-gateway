package server

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"

	"ai-gateway/internal/cache"
	"ai-gateway/internal/metrics"
	"ai-gateway/internal/middleware"
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
var startTime = time.Now()

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

func (s *Server) adminRoutes() http.Handler {
	mux := http.NewServeMux()
	ah := &AdminHandler{server: s}

	// Legacy endpoints
	mux.HandleFunc("GET /admin/health", ah.handleHealth)
	mux.HandleFunc("GET /admin/routes", ah.handleRoutes)
	mux.HandleFunc("GET /admin/cache", ah.handleCache)

	// v1 API
	mux.HandleFunc("GET /admin/api/v1/overview", s.handleAdminOverview)
	mux.HandleFunc("GET /admin/api/v1/breakers", s.handleAdminBreakers)
	mux.HandleFunc("GET /admin/api/v1/providers", s.handleAdminProviders)
	mux.HandleFunc("GET /admin/api/v1/latency", s.handleAdminLatency)
	mux.HandleFunc("GET /admin/api/v1/routes", s.handleAdminRoutesV1)
	mux.HandleFunc("GET /admin/api/v1/cache", s.handleAdminCacheV1)
	mux.HandleFunc("GET /admin/api/v1/cache/entries/", s.handleAdminCacheEntry)
	mux.HandleFunc("GET /admin/api/v1/quotas", s.handleAdminQuotas)
	mux.HandleFunc("GET /admin/api/v1/keys", s.handleAdminKeysList)
	mux.HandleFunc("POST /admin/api/v1/keys", s.handleAdminKeysCreate)
	mux.HandleFunc("PUT /admin/api/v1/keys/", s.handleAdminKeysUpdate)
	mux.HandleFunc("DELETE /admin/api/v1/keys/", s.handleAdminKeysDelete)
	mux.HandleFunc("GET /admin/api/v1/audit-logs", s.handleAdminAuditLogs)
	mux.HandleFunc("GET /admin/api/v1/filter", s.handleAdminFilter)

	return middleware.AdminOnly(mux)
}

func (ah *AdminHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

func (ah *AdminHandler) handleRoutes(w http.ResponseWriter, r *http.Request) {
	snapshot := ah.server.currentSnapshot()
	cfg := snapshot.config
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
		"routes":          routes,
		"config_revision": snapshot.revision,
	})
}

func (ah *AdminHandler) handleCache(w http.ResponseWriter, r *http.Request) {
	snapshot := ah.server.currentSnapshot()
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
		"cache_enabled":   snapshot.config.Cache.Enabled,
		"cache_backend":   snapshot.config.Cache.Backend,
		"cache_strategy":  snapshot.config.Cache.Strategy,
		"size":            cacheSize,
		"max_size":        snapshot.config.Cache.MaxSize,
		"hits":            stats.CacheHits.Load(),
		"misses":          stats.CacheMisses.Load(),
		"total_requests":  stats.TotalReqs.Load(),
		"total_errors":    stats.TotalErrors.Load(),
		"stream_requests": stats.StreamReqs.Load(),
		"hit_rate_pct":    hitRate,
		"entries":         entries,
		"config_revision": snapshot.revision,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}
