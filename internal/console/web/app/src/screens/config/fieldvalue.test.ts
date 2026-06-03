import { describe, expect, it } from 'vitest'
import { getPath, coerce } from './fieldvalue'

describe('getPath', () => {
  const eff = { fetch_proxy: { monitoring: { max_url_length: 2048 } }, mode: 'audit' }
  it('reads a nested path', () => {
    expect(getPath(eff, 'fetch_proxy.monitoring.max_url_length')).toBe(2048)
  })
  it('reads a top-level path', () => {
    expect(getPath(eff, 'mode')).toBe('audit')
  })
  it('returns undefined for an absent path', () => {
    expect(getPath(eff, 'fetch_proxy.nope.x')).toBeUndefined()
  })
})

describe('coerce', () => {
  it('coerces ints', () => { expect(coerce('int', '30')).toBe(30) })
  it('coerces floats', () => { expect(coerce('float', '4.5')).toBe(4.5) })
  it('passes bools through', () => { expect(coerce('bool', true)).toBe(true) })
  it('passes strings through', () => { expect(coerce('string', 'hi')).toBe('hi') })
})
