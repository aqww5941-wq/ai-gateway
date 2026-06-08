import { useMemo } from 'react'
import { usePolling } from '@/hooks/usePolling'
import { api } from '@/api'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { formatUptime, formatPct } from '@/utils'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts'
import { Activity, Zap, AlertTriangle, Clock, Layers, Combine } from 'lucide-react'
import type { Overview, LatencyRouteSnapshot } from '@/types'

const CACHE_COLORS = ['#10b981', '#ef4444']

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
      { name: 'Hits', value: overview.cache_hits },
      { name: 'Misses', value: overview.cache_misses },
    ].filter((d) => d.value > 0)
  }, [overview])

  if (ovLoading) return (
    <div className="flex h-[50vh] items-center justify-center">
      <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
    </div>
  )

  const metrics = [
    { title: 'Total Requests', value: overview?.total_requests ?? 0, icon: Activity, sub: 'Lifetime metric' },
    { title: 'Cache Hit Rate', value: formatPct(overview?.hit_rate_pct ?? 0), icon: Zap, sub: overview?.cache_enabled ? `${overview?.cache_strategy} strategy` : 'Disabled' },
    { title: 'Error Rate', value: formatPct(overview?.error_rate_pct ?? 0), icon: AlertTriangle, sub: `${overview?.total_errors ?? 0} total errors` },
    { title: 'Uptime', value: formatUptime(overview?.uptime_seconds ?? 0), icon: Clock, sub: 'Since last restart' },
    { title: 'Stream Requests', value: overview?.stream_requests ?? 0, icon: Layers, sub: 'SSE connections' },
    { title: 'Coalesce Ratio', value: formatPct(overview?.coalescer?.dedup_ratio_pct ?? 0), icon: Combine, sub: `${overview?.coalescer?.shared_calls ?? 0} shared` },
  ]

  return (
    <div className="space-y-6 pb-8">
      <div>
        <h2 className="text-2xl font-bold tracking-tight">Dashboard Overview</h2>
        <p className="text-muted-foreground mt-1">Real-time metrics and performance analytics.</p>
      </div>

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {metrics.map((m, i) => {
          const Icon = m.icon
          return (
            <Card key={i} className="hover:border-primary/50 transition-colors">
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardTitle className="text-sm font-medium text-muted-foreground">{m.title}</CardTitle>
                <Icon className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{m.value}</div>
                <p className="text-xs text-muted-foreground mt-1">{m.sub}</p>
              </CardContent>
            </Card>
          )
        })}
      </div>

      <div className="grid gap-4 grid-cols-1 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>P99 Latency (ms)</CardTitle>
          </CardHeader>
          <CardContent className="h-[300px]">
            {latencyChart.length > 0 ? (
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={latencyChart} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="hsl(var(--border))" />
                  <XAxis dataKey="name" tick={{ fill: 'hsl(var(--muted-foreground))', fontSize: 12 }} axisLine={false} tickLine={false} />
                  <YAxis tick={{ fill: 'hsl(var(--muted-foreground))', fontSize: 12 }} axisLine={false} tickLine={false} />
                  <Tooltip
                    contentStyle={{ backgroundColor: 'hsl(var(--card))', borderColor: 'hsl(var(--border))', borderRadius: '8px' }}
                    itemStyle={{ color: 'hsl(var(--foreground))' }}
                  />
                  <Bar dataKey="p99" fill="hsl(var(--primary))" radius={[4, 4, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            ) : (
              <div className="h-full flex items-center justify-center text-muted-foreground">No Latency Data</div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Cache Performance</CardTitle>
          </CardHeader>
          <CardContent className="h-[300px]">
            {cachePie.length > 0 ? (
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie
                    data={cachePie}
                    cx="50%"
                    cy="50%"
                    innerRadius={70}
                    outerRadius={100}
                    paddingAngle={5}
                    dataKey="value"
                    stroke="none"
                  >
                    {cachePie.map((_, index) => (
                      <Cell key={`cell-${index}`} fill={CACHE_COLORS[index % CACHE_COLORS.length]} />
                    ))}
                  </Pie>
                  <Tooltip
                    contentStyle={{ backgroundColor: 'hsl(var(--card))', borderColor: 'hsl(var(--border))', borderRadius: '8px' }}
                  />
                </PieChart>
              </ResponsiveContainer>
            ) : (
              <div className="h-full flex items-center justify-center text-muted-foreground">No Cache Data</div>
            )}
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader className="pb-3">
            <CardTitle>Cache System Status</CardTitle>
          </CardHeader>
          <CardContent>
            {overview?.cache_enabled ? (
              <div className="flex items-center gap-2">
                <Badge variant="success" className="h-6">Active</Badge>
                <span className="text-sm font-medium">{overview.cache_backend} / {overview.cache_strategy}</span>
              </div>
            ) : (
              <Badge variant="destructive" className="h-6">Disabled</Badge>
            )}
          </CardContent>
        </Card>
        
        <Card>
          <CardHeader className="pb-3">
            <CardTitle>Rate Limiter Status</CardTitle>
          </CardHeader>
          <CardContent>
            {overview?.rate_limit_enabled ? (
              <Badge variant="success" className="h-6">Enforcing</Badge>
            ) : (
              <Badge variant="secondary" className="h-6">Bypassed</Badge>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
