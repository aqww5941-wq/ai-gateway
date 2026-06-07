export interface Overview {
  uptime_seconds: number;
  total_requests: number;
  cache_hits: number;
  cache_misses: number;
  hit_rate_pct: number;
  total_errors: number;
  error_rate_pct: number;
  stream_requests: number;
  coalescer: {
    total_calls: number;
    shared_calls: number;
    dedup_ratio_pct: number;
  };
  cache_enabled: boolean;
  cache_backend: string;
  cache_strategy: string;
  rate_limit_enabled: boolean;
}

export interface BreakerSnapshot {
  name: string;
  state: string;
  failures: number;
  probes_ok: number;
}

export interface ProviderInfo {
  name: string;
  type: string;
  models: string[];
  health: string;
  breaker_state: string;
  timeout: string;
}

export interface LatencyRouteTarget {
  provider: string;
  model: string;
  p99_ms: number;
  failure_rate: number;
  samples: number;
}

export interface LatencyRouteSnapshot {
  model: string;
  route_name: string;
  targets: LatencyRouteTarget[];
}

export interface RouteInfo {
  name: string;
  match_model: string;
  strategy: string;
  targets: RouteTargetInfo[];
  semantic_rules: SemanticRuleInfo[] | null;
}

export interface RouteTargetInfo {
  provider: string;
  model: string;
  weight: number;
}

export interface SemanticRuleInfo {
  complexity: string;
  provider: string;
  model: string;
}

export interface CacheInfo {
  enabled: boolean;
  backend: string;
  strategy: string;
  max_size: number;
  current_size: number;
  hits: number;
  misses: number;
  hit_rate_pct: number;
  entries: CacheEntryInfo[];
}

export interface CacheEntryInfo {
  key: string;
  model: string;
  expires_at: string;
  token_count: number;
}

export interface CacheEntryDetail {
  key: string;
  model: string;
  expires_at: string;
  token_count: number;
  response: Record<string, unknown>;
}
