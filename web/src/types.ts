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

export interface QuotaEntry {
  key_id: number;
  name: string;
  daily_limit: number;
  monthly_limit: number;
  used_tokens: number;
  remaining_tokens: number;
  used_monthly: number;
  remaining_monthly: number;
  reset_at: number;
}

export interface QuotasResponse {
  quotas: QuotaEntry[];
  enabled: boolean;
}

export interface KeyInfo {
  id: number;
  name: string;
  role: string;
  daily_limit: number;
  monthly_limit: number;
  models: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface KeysResponse {
  keys: KeyInfo[];
}

export interface CreateKeyResponse {
  name: string;
  token: string;
}

export interface AuditEntry {
  id: number;
  key_name: string;
  model: string;
  provider: string;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  status_code: number;
  latency_ms: number;
  stream: boolean;
  error_message: string;
  created_at: string;
}

export interface AuditLogsResponse {
  logs: AuditEntry[];
  total: number;
}

export interface FilterRuleStatus {
  name: string;
  label: string;
  description: string;
  enabled: boolean;
}

export interface FilterStatus {
  enabled: boolean;
  mode: string;
  rules: FilterRuleStatus[];
}
