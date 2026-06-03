import { useMemo, useState } from 'react'
import { EVENT_STREAM_CAP, useEventStream, type PipelockEvent } from '../api'
import { eventTarget, formatTime, severityClass } from '../lib/format'
import ScreenHeader from '../components/ScreenHeader'
import Drawer, { JsonBlock } from '../components/Drawer'

type SevFilter = 'all' | 'block' | 'warn' | 'info'

// Map a raw severity onto one of the three coarse filter buckets.
function bucket(sev: string): 'block' | 'warn' | 'info' {
  const c = severityClass(sev)
  if (c === 'alert') return 'block'
  if (c === 'warn') return 'warn'
  return 'info'
}

export default function Events() {
  const { events, connected } = useEventStream()
  const [sev, setSev] = useState<SevFilter>('all')
  const [query, setQuery] = useState('')
  const [selected, setSelected] = useState<PipelockEvent | null>(null)

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    return events.filter((e) => {
      if (sev !== 'all' && bucket(e.severity) !== sev) return false
      if (!q) return true
      const hay = `${e.severity} ${e.type} ${e.pipelock_instance} ${eventTarget(e.fields)} ${JSON.stringify(e.fields)}`.toLowerCase()
      return hay.includes(q)
    })
  }, [events, sev, query])

  return (
    <div style={{ padding: '1.5rem 1.75rem', height: '100%', display: 'flex', flexDirection: 'column', minHeight: 0 }}>
      <ScreenHeader
        title="Events"
        tag="live telemetry stream"
        right={
          <span style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', fontSize: '0.72rem', color: connected ? 'var(--color-neon)' : 'var(--color-muted)' }}>
            <span style={{ width: 7, height: 7, borderRadius: '50%', background: connected ? 'var(--color-neon)' : 'var(--color-muted)', boxShadow: connected ? '0 0 7px var(--color-neon)' : 'none', animation: connected ? 'pulse 2s infinite' : 'none' }} />
            {connected ? 'streaming' : 'reconnecting…'}
          </span>
        }
      />

      {/* Controls */}
      <div style={{ display: 'flex', gap: '0.75rem', alignItems: 'center', marginBottom: '0.9rem', flexWrap: 'wrap' }}>
        <div style={{ display: 'flex', gap: '0.3rem' }}>
          {(['all', 'block', 'warn', 'info'] as SevFilter[]).map((s) => (
            <button
              key={s}
              type="button"
              onClick={() => setSev(s)}
              style={filterChip(sev === s, s)}
            >
              {s}
            </button>
          ))}
        </div>
        <input
          className="input-cyber"
          style={{ flex: 1, minWidth: '200px', maxWidth: '420px' }}
          placeholder="filter by type, target, field…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <span style={{ marginLeft: 'auto', color: 'var(--color-muted)', fontSize: '0.7rem' }}>
          {filtered.length} / {events.length} shown · last {EVENT_STREAM_CAP}
        </span>
      </div>

      {/* Table */}
      <div className="panel" style={{ flex: 1, minHeight: 0, padding: 0, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <div style={{ display: 'grid', gridTemplateColumns: '92px 1fr 1.4fr 130px', gap: '0.75rem', padding: '0.55rem 0.9rem', borderBottom: '1px solid var(--color-border)', color: 'var(--color-muted)', fontSize: '0.62rem', letterSpacing: '0.14em', textTransform: 'uppercase', flexShrink: 0, background: 'var(--color-surface)' }}>
          <span>severity</span>
          <span>type</span>
          <span>target</span>
          <span>time</span>
        </div>
        <div style={{ flex: 1, overflow: 'auto', minHeight: 0 }}>
          {filtered.length === 0 ? (
            <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--color-muted)', fontSize: '0.8rem' }}>
              {events.length === 0 ? '— awaiting events —' : '— no events match the current filter —'}
            </div>
          ) : (
            filtered.map((e, i) => {
              const cls = severityClass(e.severity)
              return (
                <button
                  key={`${e.timestamp}-${e.type}-${i}`}
                  type="button"
                  onClick={() => setSelected(e)}
                  style={{
                    width: '100%',
                    textAlign: 'left',
                    display: 'grid',
                    gridTemplateColumns: '92px 1fr 1.4fr 130px',
                    gap: '0.75rem',
                    alignItems: 'center',
                    padding: '0.5rem 0.9rem',
                    borderBottom: '1px solid var(--color-border)',
                    background: 'transparent',
                    border: 'none',
                    borderLeft: `2px solid ${cls === 'alert' ? 'var(--color-alert)' : cls === 'warn' ? 'var(--color-warn)' : 'transparent'}`,
                    cursor: 'pointer',
                    fontFamily: 'var(--font-mono)',
                    color: 'var(--color-text)',
                  }}
                  onMouseEnter={(ev) => (ev.currentTarget.style.background = 'color-mix(in srgb, var(--color-neon) 5%, transparent)')}
                  onMouseLeave={(ev) => (ev.currentTarget.style.background = 'transparent')}
                >
                  <span className={`badge badge--${cls === 'info' ? 'ok' : cls}`} style={{ fontSize: '0.56rem', justifySelf: 'start' }}>{e.severity}</span>
                  <span style={{ fontSize: '0.76rem', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{e.type}</span>
                  <span style={{ fontSize: '0.74rem', color: 'var(--color-muted)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{eventTarget(e.fields)}</span>
                  <span style={{ fontSize: '0.68rem', color: 'var(--color-muted)' }}>{formatTime(e.timestamp)}</span>
                </button>
              )
            })
          )}
        </div>
      </div>

      <Drawer
        open={selected !== null}
        title={selected?.type ?? 'event'}
        subtitle={selected ? `${selected.severity} · ${formatTime(selected.timestamp)} · ${selected.pipelock_instance || 'unknown instance'}` : undefined}
        onClose={() => setSelected(null)}
      >
        {selected && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
            <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
              <Meta label="severity" value={selected.severity} />
              <Meta label="type" value={selected.type} />
              <Meta label="instance" value={selected.pipelock_instance || '—'} />
            </div>
            <div>
              <div style={{ color: 'var(--color-muted)', fontSize: '0.64rem', letterSpacing: '0.16em', textTransform: 'uppercase', marginBottom: '0.45rem' }}>fields</div>
              <JsonBlock value={selected.fields} />
            </div>
          </div>
        )}
      </Drawer>
    </div>
  )
}

function Meta({ label, value }: { label: string; value: string }) {
  return (
    <div style={{ border: '1px solid var(--color-border)', borderRadius: 'var(--radius-panel)', padding: '0.35rem 0.6rem', minWidth: 0 }}>
      <div style={{ color: 'var(--color-muted)', fontSize: '0.56rem', letterSpacing: '0.1em', textTransform: 'uppercase' }}>{label}</div>
      <div style={{ color: 'var(--color-neon)', fontSize: '0.78rem', wordBreak: 'break-all' }}>{value}</div>
    </div>
  )
}

function filterChip(active: boolean, kind: SevFilter): React.CSSProperties {
  const color = kind === 'block' ? 'var(--color-alert)' : kind === 'warn' ? 'var(--color-warn)' : 'var(--color-neon)'
  return {
    padding: '0.35rem 0.7rem',
    fontSize: '0.68rem',
    letterSpacing: '0.06em',
    textTransform: 'uppercase',
    fontFamily: 'var(--font-mono)',
    cursor: 'pointer',
    borderRadius: 'var(--radius-panel)',
    border: `1px solid ${active ? color : 'var(--color-border)'}`,
    background: active ? `color-mix(in srgb, ${color} 14%, transparent)` : 'transparent',
    color: active ? color : 'var(--color-muted)',
  }
}
