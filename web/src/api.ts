const BASE = '/admin/api/v1';

async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, init);
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`${res.status}: ${text || res.statusText}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

async function postJSON<T>(path: string, body: unknown): Promise<T> {
  return fetchJSON<T>(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
}

async function putJSON<T>(path: string, body: unknown): Promise<T> {
  return fetchJSON<T>(path, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
}

async function deleteJSON(path: string): Promise<void> {
  await fetchJSON<void>(path, { method: 'DELETE' });
}

export const api = {
  getOverview: () =>
    fetchJSON<import('./types').Overview>('/overview'),

  getBreakers: () =>
    fetchJSON<{ breakers: import('./types').BreakerSnapshot[] }>('/breakers'),

  getProviders: () =>
    fetchJSON<{ providers: import('./types').ProviderInfo[] }>('/providers'),

  getLatency: () =>
    fetchJSON<{ routes: import('./types').LatencyRouteSnapshot[] }>('/latency'),

  getRoutes: () =>
    fetchJSON<{ routes: import('./types').RouteInfo[] }>('/routes'),

  getCache: () =>
    fetchJSON<import('./types').CacheInfo>('/cache'),

  getCacheEntry: (key: string) =>
    fetchJSON<import('./types').CacheEntryDetail>(`/cache/entries/${encodeURIComponent(key)}`),

  getQuotas: () =>
    fetchJSON<import('./types').QuotasResponse>('/quotas'),

  getKeys: () =>
    fetchJSON<import('./types').KeysResponse>('/keys'),

  createKey: (name: string, role: string, models: string, dailyLimit: number, monthlyLimit: number) =>
    postJSON<import('./types').CreateKeyResponse>('/keys', {
      name,
      role,
      models,
      daily_limit: dailyLimit,
      monthly_limit: monthlyLimit,
    }),

  updateKey: (id: number, data: { name?: string; role?: string; daily_limit?: number; monthly_limit?: number; models?: string; is_active?: boolean }) =>
    putJSON<void>(`/keys/${id}`, data),

  deleteKey: (id: number) =>
    deleteJSON(`/keys/${id}`),

  getFilterStatus: () =>
    fetchJSON<import('./types').FilterStatus>('/filter'),

  getAuditLogs: (key?: string, limit?: number, offset?: number) => {
    const params = new URLSearchParams();
    if (key) params.set('key', key);
    if (limit) params.set('limit', String(limit));
    if (offset) params.set('offset', String(offset));
    return fetchJSON<import('./types').AuditLogsResponse>(`/audit-logs?${params.toString()}`);
  },
};
