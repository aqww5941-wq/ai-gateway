import { usePolling } from '../hooks/usePolling'
import { api } from '../api'
import type { RouteInfo } from '../types'

function strategyLabel(s: string): string {
  const map: Record<string, string> = {
    round_robin: '轮询',
    weighted: '加权',
    fallback: '故障转移',
    latency: '延迟择优',
    semantic: '语义路由',
  }
  return map[s] ?? s
}

export default function RoutesPage() {
  const { data, loading, error } = usePolling<{ routes: RouteInfo[] }>(api.getRoutes, 10000)

  if (loading) return <div className="loading">加载中...</div>
  if (error) return <div className="error">{error}</div>

  const routes = data?.routes ?? []

  return (
    <div>
      <div className="section-title">路由配置 ({routes.length})</div>
      {routes.length === 0 ? (
        <div className="empty">暂无路由配置</div>
      ) : (
        <div className="card" style={{ padding: 0, overflow: 'auto' }}>
          <table className="data-table">
            <thead>
              <tr>
                <th>名称</th>
                <th>匹配模型</th>
                <th>策略</th>
                <th>目标</th>
                <th>语义规则</th>
              </tr>
            </thead>
            <tbody>
              {routes.map((r) => (
                <tr key={r.name}>
                  <td><strong>{r.name}</strong></td>
                  <td><code>{r.match_model}</code></td>
                  <td>
                    <span className={`badge badge-${r.strategy}`}>{strategyLabel(r.strategy)}</span>
                  </td>
                  <td>
                    {r.targets.length > 0
                      ? r.targets.map((t) => (
                          <div key={t.provider + t.model} style={{ marginBottom: 2 }}>
                            {t.provider} / <code>{t.model}</code>
                            {t.weight > 0 && ` (w:${t.weight})`}
                          </div>
                        ))
                      : '-'}
                  </td>
                  <td>
                    {r.semantic_rules && r.semantic_rules.length > 0
                      ? r.semantic_rules.map((sr) => (
                          <div key={sr.complexity} style={{ marginBottom: 2 }}>
                            <span className={`badge badge-${sr.complexity === 'simple' ? 'healthy' : 'degraded'}`}>
                              {sr.complexity === 'simple' ? '简单' : '复杂'}
                            </span>
                            {' → '}
                            {sr.provider} / <code>{sr.model}</code>
                          </div>
                        ))
                      : '-'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
