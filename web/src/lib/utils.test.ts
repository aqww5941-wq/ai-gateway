import { describe, expect, it } from 'vitest'

import { cn } from './utils'

describe('cn', () => {
  it('filters conditional values and resolves Tailwind conflicts', () => {
    expect(cn('px-2 text-sm', null, 'px-4')).toBe('text-sm px-4')
  })
})
