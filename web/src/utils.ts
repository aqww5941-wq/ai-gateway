export function formatUptime(s: number): string {
    const d = Math.floor(s / 86400)
    const h = Math.floor((s % 86400) / 3600)
    const m = Math.floor((s % 3600) / 60)
    if (d > 0) return `${d}天 ${h}时 ${m}分`
    if (h > 0) return `${h}时 ${m}分`
    return `${m}分`
}

export function formatPct(v: number): string {
    return v.toFixed(1) + '%'
}

export function strategyLabel(s: string): string {
    const map: Record<string, string> = {
        round_robin: '轮询',
        weighted: '加权',
        fallback: '故障转移',
        latency: '延迟择优',
        semantic: '语义路由',
    }
    return map[s] ?? s
}

export function providerTypeLabel(t: string): string {
    if (t === 'openai') return 'OpenAI 兼容'
    if (t === 'claude') return 'Claude'
    return t
}

export function formatDate(isoString: string): string {
    return new Date(isoString).toLocaleString('zh-CN')
}