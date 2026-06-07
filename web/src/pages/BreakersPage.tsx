import { usePolling } from '../hooks/usePolling'
import { api } from '../api'
import StatusBadge from '../components/StatusBadge'
import type { BreakerSnapshot } from '../types'

export default function BreakersPage() {
  const { data, loading, error } = usePolling<{ breakers: BreakerSnapshot[] }>(api.getBreakers, 3000)

  if (loading) return <div className="loading">加载中...</div>
  if (error) return <div className="error">{error}</div>

  const breakers = data?.breakers ?? []

  return (
    <div>
      <div className="section-title">熔断器状态 ({breakers.length})</div>
      {breakers.length === 0 ? (
        <div className="empty">暂无熔断器数据 — 需要先发起请求</div>
      ) : (
        <div className="card" style={{ padding: 0, overflow: 'auto' }}>
          <table className="data-table">
            <thead>
              <tr>
                <th>提供商</th>
                <th>状态</th>
                <th>故障次数</th>
                <th>探测成功</th>
              </tr>
            </thead>
            <tbody>
              {breakers.map((b) => (
                <tr key={b.name}>
                  <td><strong>{b.name}</strong></td>
                  <td>
                    <StatusBadge status={b.state} />
                  </td>
                  <td>{b.failures}</td>
                  <td>{b.probes_ok}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
