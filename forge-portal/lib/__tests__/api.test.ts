import { describe, it, expect, vi, beforeEach } from 'vitest'

describe('api module', () => {
  it('should be importable', async () => {
    const mod = await import('../api')
    expect(mod.api).toBeDefined()
    expect(mod.api.get).toBeTypeOf('function')
    expect(mod.api.post).toBeTypeOf('function')
    expect(mod.api.put).toBeTypeOf('function')
    expect(mod.api.delete).toBeTypeOf('function')
  })
})
