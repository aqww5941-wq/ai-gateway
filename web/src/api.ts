const BASE = '/admin/api/v1';

async function fetchJSON<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`);
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
  return res.json();
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
};
