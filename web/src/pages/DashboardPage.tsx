import { useMemo } from 'react'
import { usePolling } from '../hooks/usePolling'
import { api } from '../api'
import MetricCard from '../components/MetricCard'
import Card from '../components/Card'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts'
import type { Overview, LatencyRouteSnapshot } from '../types'

function formatUptime(s: number): string {
  const d = Math.floor(s / 86400)
  const h = Math.floor((s % 86400) / 3600)
  const m = Math.floor((s % 3600) / 60)
  if (d > 0) return `${d}天 ${h}时 ${m}分`
  if (h > 0) return `${h}时 ${m}分`
  return `${m}分`
}

function formatPct(v: number): string {
  return v.toFixed(1) + '%'
}

const CACHE_COLORS = ['#22c55e', '#ef4444']

export default function DashboardPage() {
  const { data: overview, loading: ovLoading } = usePolling<Overview>(api.getOverview, 5000)
  const { data: latencyData } = usePolling<{ routes: LatencyRouteSnapshot[] }>(api.getLatency, 5000)

  const latencyChart = useMemo(() => {
    if (!latencyData?.routes) return []
    return latencyData.routes.flatMap((r) =>
      r.targets.map((t) => ({
        name: `${t.provider}/${t.model}`,
        p99: Math.round(t.p99_ms),
        route: r.route_name,
      }))
    )
  }, [latencyData])

  const cachePie = useMemo(() => {
    if (!overview) return []
    return [
      { name: '命中', value: overview.cache_hits },
      { name: '未命中', value: overview.cache_misses },
    ].filter((d) => d.value > 0)
  }, [overview])

  if (ovLoading) return <div className="loading">加载中...</div>

  return (
    <div>
      <div className="metric-grid">
        <MetricCard label="总请求数" value={overview?.total_requests ?? 0} />
        <MetricCard
          label="缓存命中率"
          value={formatPct(overview?.hit_rate_pct ?? 0)}
          sub={overview?.cache_enabled ? (overview?.cache_strategy ?? '') + ' 策略' : '已禁用'}
        />
        <MetricCard
          label="错误率"
          value={formatPct(overview?.error_rate_pct ?? 0)}
          sub={`${overview?.total_errors ?? 0} 次错误`}
        />
        <MetricCard label="运行时长" value={formatUptime(overview?.uptime_seconds ?? 0)} />
        <MetricCard
          label="流式请求数"
          value={overview?.stream_requests ?? 0}
        />
        <MetricCard
          label="请求合并率"
          value={formatPct(overview?.coalescer?.dedup_ratio_pct ?? 0)}
          sub={`${overview?.coalescer?.shared_calls ?? 0} 共享 / ${overview?.coalescer?.total_calls ?? 0} 总计`}
        />
      </div>

      <div className="charts-grid">
        <Card title="P99 延迟 (ms)">
          <div className="chart-container">
            {latencyChart.length > 0 ? (
              <ResponsiveContainer width="100%" height={300}>
                <BarChart data={latencyChart} margin={{ top: 8, right: 8, left: 8, bottom: 8 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#2a2d3a" />
                  <XAxis dataKey="name" tick={{ fill: '#6b6f82', fontSize: 11 }} />
                  <YAxis tick={{ fill: '#6b6f82', fontSize: 11 }} />
                  <Tooltip
                    contentStyle={{ background: '#1e2130', border: '1px solid #2a2d3a', borderRadius: 8 }}
                    labelStyle={{ color: '#e4e6eb' }}
                  />
                  <Bar dataKey="p99" fill="#6366f1" radius={[4, 4, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            ) : (
              <div className="empty">暂无延迟数据</div>
            )}
          </div>
        </Card>

        <Card title="缓存表现">
          <div className="chart-container">
            {cachePie.length > 0 ? (
              <ResponsiveContainer width="100%" height={300}>
                <PieChart>
                  <Pie
                    data={cachePie}
                    cx="50%"
                    cy="50%"
                    innerRadius={60}
                    outerRadius={100}
                    dataKey="value"
                    label={({ name, value }) => `${name}: ${value}`}
                  >
                    {cachePie.map((_, i) => (
                      <Cell key={i} fill={CACHE_COLORS[i]} />
                    ))}
                  </Pie>
                  <Tooltip
                    contentStyle={{ background: '#1e2130', border: '1px solid #2a2d3a', borderRadius: 8 }}
                  />
                </PieChart>
              </ResponsiveContainer>
            ) : (
              <div className="empty">暂无缓存数据</div>
            )}
          </div>
        </Card>
      </div>

      <div className="metric-grid">
        <Card title="缓存状态">
          <div style={{ fontSize: 14 }}>
            {overview?.cache_enabled ? (
              <span className="badge badge-healthy" style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
                <span className="badge-dot" style={{ background: 'var(--success)' }} />
                {overview.cache_backend} / {overview.cache_strategy}
              </span>
            ) : (
              <span className="badge badge-unhealthy" style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
                <span className="badge-dot" style={{ background: 'var(--danger)' }} />
                已禁用
              </span>
            )}
          </div>
        </Card>
        <Card title="限流状态">
          <div style={{ fontSize: 14 }}>
            {overview?.rate_limit_enabled ? (
              <span className="badge badge-healthy" style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
                <span className="badge-dot" style={{ background: 'var(--success)' }} />
                已启用
              </span>
            ) : (
              <span className="badge badge-degraded" style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
                <span className="badge-dot" style={{ background: 'var(--text-muted)' }} />
                已禁用
              </span>
            )}
          </div>
        </Card>
      </div>
    </div>
  )
}
