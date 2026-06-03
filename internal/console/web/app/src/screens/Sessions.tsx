import { useState } from 'react'
import { getSessions } from '../api'
import { usePolling } from '../lib/usePolling'
import { pickString } from '../lib/format'
import ScreenHeader from '../components/ScreenHeader'
import Banner from '../components/Banner'
import Drawer, { JsonBlock } from '../components/Drawer'

type Session = Record<string, unknown>

// Pull a short display label for a session row, trying common id-ish keys.
function sessionLabel(s: Session): string {
  return pickString(s, 'key', 'id', 'session_id', 'session', 'name', 'agent', 'client') ?? '(unkeyed session)'
}

function tierOf(s: Session): string | undefined {
  return pickString(s, 'tier', 'level', 'profile')
}

function escalationOf(s: Session): string | undefined {
  return pickString(s, 'escalation', 'escalated', 'enforcement', 'state', 'status')
}

function anomalyOf(s: Session): string | undefined {
  return pickString(s, 'anomalies', 'anomaly_count', 'anomaly', 'risk', 'score')
}

export default function Sessions() {
  const { data, error, loading, refresh } = usePolling(getSessions, 5000)
  const [selected, setSelected] = useState<Session | null>(null)

  const sessions = data?.sessions ?? []

  return (
    <div style={{ padding: '1.5rem 1.75rem', height: '100%', display: 'flex', flexDirection: 'column', minHeight: 0 }}>
      <ScreenHeader
        title="Sessions"
        tag="active agent sessions"
        right={<span style={{ color: 'var(--color-muted)', fontSize: '0.72rem' }}>{data ? `${data.count} tracked` : '—'}</span>}
      />

      {error && (
        <div style={{ marginBottom: '1rem' }}>
          <Banner tone="alert" onRetry={refresh}>
            Cannot reach the sessions endpoint — the proxy may be down. Showing last known data.
          </Banner>
        </div>
      )}

      <div className="panel" style={{ flex: 1, minHeight: 0, padding: 0, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr 1fr 1fr 80px', gap: '0.75rem', padding: '0.55rem 0.9rem', borderBottom: '1px solid var(--color-border)', color: 'var(--color-muted)', fontSize: '0.62rem', letterSpacing: '0.14em', textTransform: 'uppercase', flexShrink: 0, background: 'var(--color-surface)' }}>
          <span>session</span>
          <span>tier</span>
          <span>escalation</span>
          <span>anomaly</span>
          <span>fields</span>
        </div>
        <div style={{ flex: 1, overflow: 'auto', minHeight: 0 }}>
          {sessions.length === 0 ? (
            <div style={{ padding: '2.5rem', textAlign: 'center', color: 'var(--color-muted)', fontSize: '0.82rem' }}>
              {loading && !data ? '— loading sessions —' : '— no active sessions —'}
            </div>
          ) : (
            sessions.map((s, i) => {
              const tier = tierOf(s)
              const esc = escalationOf(s)
              const escalated = esc !== undefined && /true|escalat|block|strict|deny/i.test(esc)
              return (
                <button
                  key={`${sessionLabel(s)}-${i}`}
                  type="button"
                  onClick={() => setSelected(s)}
                  style={{
                    width: '100%',
                    textAlign: 'left',
                    display: 'grid',
                    gridTemplateColumns: '2fr 1fr 1fr 1fr 80px',
                    gap: '0.75rem',
                    alignItems: 'center',
                    padding: '0.55rem 0.9rem',
                    borderBottom: '1px solid var(--color-border)',
                    borderLeft: `2px solid ${escalated ? 'var(--color-alert)' : 'transparent'}`,
                    background: 'transparent',
                    border: 'none',
                    cursor: 'pointer',
                    fontFamily: 'var(--font-mono)',
                    color: 'var(--color-text)',
                  }}
                  onMouseEnter={(ev) => (ev.currentTarget.style.background = 'color-mix(in srgb, var(--color-neon) 5%, transparent)')}
                  onMouseLeave={(ev) => (ev.currentTarget.style.background = 'transparent')}
                >
                  <span style={{ fontSize: '0.78rem', color: 'var(--color-neon)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{sessionLabel(s)}</span>
                  <span style={{ fontSize: '0.74rem', color: 'var(--color-muted)' }}>{tier ?? '—'}</span>
                  <span style={{ fontSize: '0.74rem' }}>
                    {esc !== undefined ? <span className={`badge badge--${escalated ? 'alert' : 'ok'}`} style={{ fontSize: '0.56rem' }}>{esc}</span> : <span style={{ color: 'var(--color-muted)' }}>—</span>}
                  </span>
                  <span style={{ fontSize: '0.74rem', color: 'var(--color-muted)' }}>{anomalyOf(s) ?? '—'}</span>
                  <span style={{ fontSize: '0.68rem', color: 'var(--color-muted)', justifySelf: 'end' }}>{Object.keys(s).length} ▸</span>
                </button>
              )
            })
          )}
        </div>
      </div>

      <Drawer
        open={selected !== null}
        title="session detail"
        subtitle={selected ? sessionLabel(selected) : undefined}
        onClose={() => setSelected(null)}
      >
        {selected && <JsonBlock value={selected} />}
      </Drawer>
    </div>
  )
}
