// Small formatting helpers shared across screens.

export function formatUptime(seconds: number): string {
  if (!isFinite(seconds) || seconds < 0) return '—'
  const s = Math.floor(seconds)
  const d = Math.floor(s / 86400)
  const h = Math.floor((s % 86400) / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  const parts: string[] = []
  if (d) parts.push(`${d}d`)
  if (h || d) parts.push(`${h}h`)
  if (m || h || d) parts.push(`${m}m`)
  parts.push(`${sec}s`)
  return parts.join(' ')
}

export function formatNumber(n: number): string {
  if (!isFinite(n)) return '—'
  return n.toLocaleString('en-US')
}

export function formatPercent(rate: number): string {
  if (!isFinite(rate)) return '—'
  return `${(rate * 100).toFixed(1)}%`
}

// Best-effort timestamp formatting; falls back to the raw string.
export function formatTime(ts: string): string {
  if (!ts) return '—'
  const d = new Date(ts)
  if (isNaN(d.getTime())) return ts
  return d.toLocaleTimeString('en-US', { hour12: false }) + '.' + String(d.getMilliseconds()).padStart(3, '0')
}

export function severityClass(sev: string): 'alert' | 'warn' | 'ok' | 'info' {
  const s = sev.toLowerCase()
  if (s === 'critical' || s === 'high' || s === 'block' || s === 'error' || s === 'alert') return 'alert'
  if (s === 'warn' || s === 'warning' || s === 'medium') return 'warn'
  if (s === 'info' || s === 'low' || s === 'debug') return 'info'
  return 'ok'
}

// Pull a human "target" out of an event's fields, trying common keys.
export function eventTarget(fields: Record<string, unknown>): string {
  const keys = ['host', 'domain', 'target', 'url', 'destination', 'dest', 'tool', 'tool_name', 'path', 'session', 'reason']
  for (const k of keys) {
    const v = fields[k]
    if (typeof v === 'string' && v) return v
    if (typeof v === 'number') return String(v)
  }
  // Fall back to first stringy field.
  for (const v of Object.values(fields)) {
    if (typeof v === 'string' && v) return v
  }
  return '—'
}

// Defensive accessor for a string field from an unknown record.
export function pickString(obj: Record<string, unknown>, ...keys: string[]): string | undefined {
  for (const k of keys) {
    const v = obj[k]
    if (typeof v === 'string' && v) return v
    if (typeof v === 'number' || typeof v === 'boolean') return String(v)
  }
  return undefined
}
