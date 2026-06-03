import { describe, expect, it } from 'vitest'
import { getPath, setPath, coerce } from './fieldvalue'

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

describe('setPath', () => {
  it('sets a top-level key returning a new object', () => {
    const src = { a: 1, b: 2 }
    const out = setPath(src, 'a', 9)
    expect(out).toEqual({ a: 9, b: 2 })
    expect(out).not.toBe(src)
    expect(src.a).toBe(1) // original untouched
  })

  it('sets a nested path, cloning along the way and leaving siblings intact', () => {
    const src = { outer: { keep: 'me', inner: { x: 1 } }, other: 'sib' }
    const out = setPath(src, 'outer.inner.x', 42)
    expect(out.other).toBe('sib')
    expect((out.outer as Record<string, unknown>).keep).toBe('me')
    expect(getPath(out, 'outer.inner.x')).toBe(42)
    // immutability: new object identity at each touched level
    expect(out).not.toBe(src)
    expect(out.outer).not.toBe(src.outer)
    expect(getPath(src, 'outer.inner.x')).toBe(1)
  })

  it('creates missing intermediate objects', () => {
    const out = setPath({}, 'a.b.c', 'v')
    expect(getPath(out, 'a.b.c')).toBe('v')
  })
})

describe('coerce', () => {
  it('coerces ints', () => { expect(coerce('int', '30')).toBe(30) })
  it('coerces floats', () => { expect(coerce('float', '4.5')).toBe(4.5) })
  it('passes bools through', () => { expect(coerce('bool', true)).toBe(true) })
  it('passes strings through', () => { expect(coerce('string', 'hi')).toBe('hi') })
})
