import { describe, expect, it } from 'vitest'

import { formatPct, formatUptime, providerTypeLabel, strategyLabel } from './utils'

describe('formatUptime', () => {
  it.each([
    { seconds: 59, expected: '0分' },
    { seconds: 3_660, expected: '1时 1分' },
    { seconds: 90_060, expected: '1天 1时 1分' },
  ])('formats $seconds seconds', ({ seconds, expected }) => {
    expect(formatUptime(seconds)).toBe(expected)
  })
})

describe('admin labels', () => {
  it('formats percentages deterministically', () => {
    expect(formatPct(12.34)).toBe('12.3%')
  })

  it('maps known values and preserves unknown values', () => {
    expect(strategyLabel('fallback')).toBe('故障转移')
    expect(strategyLabel('future-strategy')).toBe('future-strategy')
    expect(providerTypeLabel('openai')).toBe('OpenAI 兼容')
    expect(providerTypeLabel('future-provider')).toBe('future-provider')
  })
})
