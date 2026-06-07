import { useState, useCallback } from 'react'
import { usePolling } from '../hooks/usePolling'
import { api } from '../api'
import MetricCard from '../components/MetricCard'
import type { CacheInfo, CacheEntryDetail } from '../types'

function formatPct(v: number): string {
  return v.toFixed(1) + '%'
}

export default function CachePage() {
  const { data, loading } = usePolling<CacheInfo>(api.getCache, 5000)
  const [detail, setDetail] = useState<CacheEntryDetail | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailError, setDetailError] = useState<string | null>(null)

  const viewEntry = useCallback(async (key: string) => {
    setDetailLoading(true)
    setDetailError(null)
    try {
      const d = await api.getCacheEntry(key)
      setDetail(d)
    } catch (e: unknown) {
      setDetailError((e as Error).message)
    } finally {
      setDetailLoading(false)
    }
  }, [])

  const closeDetail = useCallback(() => {
    setDetail(null)
    setDetailError(null)
  }, [])

  if (loading) return <div className="loading">加载中...</div>

  return (
    <div>
      <div className="metric-grid">
        <MetricCard label="命中率" value={formatPct(data?.hit_rate_pct ?? 0)} />
        <MetricCard label="当前条目数" value={data?.current_size ?? 0} sub={`最大 ${data?.max_size ?? 0}`} />
        <MetricCard label="命中次数" value={data?.hits ?? 0} />
        <MetricCard label="未命中次数" value={data?.misses ?? 0} />
      </div>

      <div className="section">
        <div className="section-title">
          缓存条目 ({(data?.entries ?? []).length})
        </div>
        {(data?.entries ?? []).length === 0 ? (
          <div className="empty">暂无缓存条目</div>
        ) : (
          <div className="card" style={{ padding: 0, overflow: 'auto' }}>
            <table className="data-table">
              <thead>
                <tr>
                  <th>Key</th>
                  <th>模型</th>
                  <th>过期时间</th>
                  <th>Token 数</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {data!.entries.map((e) => (
                  <tr key={e.key}>
                    <td>
                      <code title={e.key}>{e.key.slice(0, 16)}...</code>
                    </td>
                    <td>{e.model}</td>
                    <td>{new Date(e.expires_at).toLocaleString('zh-CN')}</td>
                    <td>{e.token_count}</td>
                    <td>
                      <button className="btn" onClick={() => viewEntry(e.key)}>
                        查看
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {detail && (
        <div className="modal-overlay" onClick={closeDetail}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h3>缓存条目: {detail.key.slice(0, 16)}...</h3>
              <button className="modal-close" onClick={closeDetail}>&times;</button>
            </div>
            <div style={{ fontSize: 13, color: 'var(--text-secondary)', marginBottom: 12 }}>
              模型: {detail.model} &middot;
              过期时间: {new Date(detail.expires_at).toLocaleString('zh-CN')} &middot;
              Token: {detail.token_count}
            </div>
            <pre>{JSON.stringify(detail.response, null, 2)}</pre>
          </div>
        </div>
      )}

      {(detailLoading || detailError) && (
        <div className="modal-overlay" onClick={closeDetail}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h3>缓存条目</h3>
              <button className="modal-close" onClick={closeDetail}>&times;</button>
            </div>
            {detailLoading && <div className="loading">加载中...</div>}
            {detailError && <div className="error">{detailError}</div>}
          </div>
        </div>
      )}
    </div>
  )
}
