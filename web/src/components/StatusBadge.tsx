const labelMap: Record<string, string> = {
  healthy: '健康',
  degraded: '降级',
  unhealthy: '异常',
  closed: '闭合',
  open: '断开',
  'half-open': '半开',
}

const dotColors: Record<string, string> = {
  healthy: 'var(--success)',
  degraded: 'var(--warning)',
  unhealthy: 'var(--danger)',
  closed: 'var(--success)',
  open: 'var(--danger)',
  'half-open': 'var(--warning)',
}

export default function StatusBadge({ status }: { status: string }) {
  const cls = `badge badge-${status}`
  const dotColor = dotColors[status]
  const label = labelMap[status] ?? status
  return (
    <span className={cls}>
      {dotColor && (
        <span className="badge-dot" style={{ background: dotColor }} />
      )}
      {label}
    </span>
  )
}
