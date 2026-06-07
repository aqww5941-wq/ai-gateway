import { usePolling } from '../hooks/usePolling'
import { api } from '../api'
import StatusBadge from '../components/StatusBadge'
import type { ProviderInfo } from '../types'

function providerTypeLabel(t: string): string {
  if (t === 'openai') return 'OpenAI 兼容'
  if (t === 'claude') return 'Claude'
  return t
}

export default function ProvidersPage() {
  const { data, loading, error } = usePolling<{ providers: ProviderInfo[] }>(api.getProviders, 10000)

  if (loading) return <div className="loading">加载中...</div>
  if (error) return <div className="error">{error}</div>

  const providers = data?.providers ?? []

  return (
    <div>
      <div className="section-title">提供商 ({providers.length})</div>
      {providers.length === 0 ? (
        <div className="empty">暂无提供商配置</div>
      ) : (
        <div className="provider-grid">
          {providers.map((p) => (
            <div key={p.name} className="card provider-card">
              <div className="provider-name">{p.name}</div>
              <div className="provider-meta">
                类型: {providerTypeLabel(p.type)} &middot; 超时: {p.timeout}
              </div>
              <div style={{ marginBottom: 8 }}>
                健康: <StatusBadge status={p.health} />
                {' '}
                熔断器: <StatusBadge status={p.breaker_state} />
              </div>
              <div className="model-tags">
                {p.models.map((m) => (
                  <span key={m} className="model-tag">{m}</span>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
