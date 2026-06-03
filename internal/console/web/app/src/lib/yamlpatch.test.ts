import { describe, expect, it } from 'vitest'
import { readSequence, addToSequence, removeFromSequence } from './yamlpatch'

const nested = `mode: audit
fetch_proxy:
  listen: "127.0.0.1:8888"
  monitoring:
    max_url_length: 2048
    blocklist:
      - "*.pastebin.com"   # keep this comment
      - "*.file.io"
forward_proxy:
  enabled: true
`

describe('readSequence', () => {
  it('reads a nested sequence', () => {
    expect(readSequence(nested, 'fetch_proxy.monitoring.blocklist')).toEqual(['*.pastebin.com', '*.file.io'])
  })
  it('returns [] for an absent key', () => {
    expect(readSequence(nested, 'ssrf.ip_allowlist')).toEqual([])
  })
})

describe('removeFromSequence', () => {
  it('removes an item and preserves the rest + comments', () => {
    const out = removeFromSequence(nested, 'fetch_proxy.monitoring.blocklist', '*.file.io')
    expect(readSequence(out, 'fetch_proxy.monitoring.blocklist')).toEqual(['*.pastebin.com'])
    expect(out).toContain('# keep this comment')
    expect(out).toContain('forward_proxy:')
  })
  it('is a no-op for an absent item', () => {
    expect(removeFromSequence(nested, 'fetch_proxy.monitoring.blocklist', 'nope')).toBe(nested)
  })
})

describe('addToSequence', () => {
  it('appends to an existing list and is idempotent', () => {
    const once = addToSequence(nested, 'fetch_proxy.monitoring.blocklist', '*.evil.com')
    expect(readSequence(once, 'fetch_proxy.monitoring.blocklist')).toContain('*.evil.com')
    expect(addToSequence(once, 'fetch_proxy.monitoring.blocklist', '*.evil.com')).toBe(once)
  })
  it('creates a brand-new top-level section when absent', () => {
    const out = addToSequence(nested, 'ssrf.ip_allowlist', '10.1.2.3/32')
    expect(readSequence(out, 'ssrf.ip_allowlist')).toEqual(['10.1.2.3/32'])
    expect(out).toContain('mode: audit')
    expect(out).toContain('# keep this comment')
  })
  it('adds to an existing top-level scalar-free list (api_allowlist: [])', () => {
    // api_allowlist is often written inline-empty; adding should still work
    const base = 'mode: audit\napi_allowlist: []\n'
    const out = addToSequence(base, 'api_allowlist', 'github.com')
    expect(readSequence(out, 'api_allowlist')).toContain('github.com')
  })
  it('handles inline-empty list with a trailing comment', () => {
    const base = 'mode: audit\napi_allowlist: []   # nothing yet\n'
    const out = addToSequence(base, 'api_allowlist', 'github.com')
    expect(readSequence(out, 'api_allowlist')).toEqual(['github.com'])
    expect(out).toContain('# nothing yet')
  })

  it('merges into an existing parent rather than duplicating it (child key missing)', () => {
    const base = 'mode: audit\nfetch_proxy:\n  listen: "127.0.0.1:8888"\n'
    const out = addToSequence(base, 'fetch_proxy.monitoring.blocklist', '*.evil.com')
    expect(readSequence(out, 'fetch_proxy.monitoring.blocklist')).toEqual(['*.evil.com'])
    // exactly one top-level fetch_proxy — no duplicate that would override or break YAML
    expect(out.match(/^fetch_proxy:/gm)?.length).toBe(1)
    expect(out).toContain('listen: "127.0.0.1:8888"')
  })

  it('adds a missing leaf list under an existing intermediate parent', () => {
    const base = 'fetch_proxy:\n  monitoring:\n    max_url_length: 2048\n'
    const out = addToSequence(base, 'fetch_proxy.monitoring.blocklist', '*.evil.com')
    expect(readSequence(out, 'fetch_proxy.monitoring.blocklist')).toEqual(['*.evil.com'])
    expect(out.match(/monitoring:/g)?.length).toBe(1)
    expect(out).toContain('max_url_length: 2048')
  })
})
