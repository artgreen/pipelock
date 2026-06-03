import { getStats, useEventStream, type NamedCount } from '../api'
import { usePolling } from '../lib/usePolling'
import { formatNumber, formatPercent, formatTime, formatUptime, eventTarget, severityClass } from '../lib/format'
import ScreenHeader from '../components/ScreenHeader'
import Banner from '../components/Banner'

export default function Overview() {
  const { data: stats, error, loading, refresh } = usePolling(getStats, 5000)
  const { events, connected } = useEventStream()

  const req = stats?.requests
  const sess = stats?.sessions

  return (
    <div style={{ padding: '1.5rem 1.75rem' }}>
      <ScreenHeader
        title="Overview"
        tag="situational awareness"
        right={
          stats && (
            <span style={{ color: 'var(--color-muted)', fontSize: '0.72rem' }}>
              uptime <span style={{ color: 'var(--color-neon)' }}>{formatUptime(stats.uptime_seconds)}</span>
            </span>
          )
        }
      />

      {error && (
        <div style={{ marginBottom: '1rem' }}>
          <Banner tone="alert" onRetry={refresh}>
            Cannot reach pipelock stats endpoint — the proxy may be down or restarting. Showing last known values.
          </Banner>
        </div>
      )}

      {/* Counter cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(165px, 1fr))', gap: '0.9rem', marginBottom: '1.25rem' }}>
        <StatCard label="requests total" value={req ? formatNumber(req.total) : '—'} loading={loading} />
        <StatCard label="blocked" value={req ? formatNumber(req.blocked) : '—'} tone="alert" loading={loading} />
        <StatCard label="block rate" value={req ? formatPercent(req.block_rate) : '—'} tone={req && req.block_rate > 0.25 ? 'alert' : 'neon'} loading={loading} />
        <StatCard label="active sessions" value={sess ? formatNumber(sess.active) : '—'} loading={loading} />
        <StatCard label="anomalies" value={sess ? formatNumber(sess.anomalies) : '—'} tone={sess && sess.anomalies > 0 ? 'warn' : 'neon'} loading={loading} />
        <StatCard label="escalations" value={sess ? formatNumber(sess.escalations) : '—'} tone={sess && sess.escalations > 0 ? 'alert' : 'neon'} loading={loading} />
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) minmax(0, 1fr)', gap: '1rem', alignItems: 'start' }}>
        {/* Blocks by layer / scanner */}
        <div className="panel" style={{ minWidth: 0 }}>
          <PanelTitle>top scanners — blocks by layer</PanelTitle>
          <RankedList items={stats?.top_scanners ?? null} tone="neon" empty="no scanner blocks recorded" />
          <hr className="divider" />
          <PanelTitle>top blocked domains</PanelTitle>
          <RankedList items={stats?.top_blocked_domains ?? null} tone="alert" empty="no blocked domains recorded" />
        </div>

        {/* Live recent events */}
        <div className="panel" style={{ minWidth: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <PanelTitle noMargin>recent events</PanelTitle>
            <span style={{ display: 'flex', alignItems: 'center', gap: '0.35rem', fontSize: '0.66rem', color: connected ? 'var(--color-neon)' : 'var(--color-muted)' }}>
              <span style={{ width: 6, height: 6, borderRadius: '50%', background: connected ? 'var(--color-neon)' : 'var(--color-muted)', boxShadow: connected ? '0 0 6px var(--color-neon)' : 'none' }} />
              {connected ? 'live' : 'offline'}
            </span>
          </div>
          <div style={{ marginTop: '0.75rem', display: 'flex', flexDirection: 'column', gap: '0.35rem', maxHeight: '420px', overflow: 'auto' }}>
            {events.length === 0 ? (
              <div style={{ color: 'var(--color-muted)', fontSize: '0.78rem', padding: '0.5rem 0' }}>— awaiting events —</div>
            ) : (
              events.slice(0, 30).map((e, i) => {
                const cls = severityClass(e.severity)
                return (
                  <div key={`${e.timestamp}-${i}`} style={{ display: 'flex', gap: '0.6rem', fontSize: '0.74rem', alignItems: 'baseline', borderBottom: '1px solid var(--color-border)', paddingBottom: '0.3rem' }}>
                    <span className={`badge badge--${cls === 'info' ? 'ok' : cls}`} style={{ fontSize: '0.58rem', flexShrink: 0 }}>{e.severity}</span>
                    <span style={{ color: 'var(--color-text)', flexShrink: 0 }}>{e.type}</span>
                    <span style={{ color: 'var(--color-muted)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', flex: 1 }}>{eventTarget(e.fields)}</span>
                    <span style={{ color: 'var(--color-muted)', fontSize: '0.66rem', flexShrink: 0 }}>{formatTime(e.timestamp)}</span>
                  </div>
                )
              })
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

function PanelTitle({ children, noMargin }: { children: React.ReactNode; noMargin?: boolean }) {
  return (
    <div style={{ color: 'var(--color-muted)', fontSize: '0.64rem', letterSpacing: '0.18em', textTransform: 'uppercase', marginBottom: noMargin ? 0 : '0.6rem' }}>
      {children}
    </div>
  )
}

function StatCard({ label, value, tone = 'neon', loading }: { label: string; value: string; tone?: 'neon' | 'alert' | 'warn'; loading?: boolean }) {
  const color = tone === 'alert' ? 'var(--color-alert)' : tone === 'warn' ? 'var(--color-warn)' : 'var(--color-neon)'
  return (
    <div className="panel" style={{ position: 'relative', overflow: 'hidden' }}>
      <div style={{ position: 'absolute', top: 0, left: 0, width: '100%', height: '2px', background: color, opacity: 0.55 }} />
      <div style={{ color: 'var(--color-muted)', fontSize: '0.62rem', letterSpacing: '0.14em', textTransform: 'uppercase' }}>{label}</div>
      <div style={{ fontSize: '1.85rem', fontWeight: 800, color, marginTop: '0.3rem', textShadow: `0 0 12px color-mix(in srgb, ${color} 40%, transparent)`, opacity: loading && value === '—' ? 0.4 : 1, fontVariantNumeric: 'tabular-nums' }}>
        {value}
      </div>
    </div>
  )
}

function RankedList({ items, tone, empty }: { items: NamedCount[] | null; tone: 'neon' | 'alert'; empty: string }) {
  if (!items || items.length === 0) {
    return <div style={{ color: 'var(--color-muted)', fontSize: '0.76rem' }}>{empty}</div>
  }
  const max = Math.max(...items.map((i) => i.count), 1)
  const color = tone === 'alert' ? 'var(--color-alert)' : 'var(--color-neon)'
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '0.45rem' }}>
      {items.map((it) => (
        <div key={it.name} style={{ position: 'relative' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.76rem', marginBottom: '0.2rem', position: 'relative', zIndex: 1 }}>
            <span style={{ color: 'var(--color-text)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '70%' }}>{it.name}</span>
            <span style={{ color, fontVariantNumeric: 'tabular-nums' }}>{formatNumber(it.count)}</span>
          </div>
          <div style={{ height: '3px', background: 'var(--color-border)', borderRadius: '2px', overflow: 'hidden' }}>
            <div style={{ width: `${(it.count / max) * 100}%`, height: '100%', background: color, boxShadow: `0 0 6px ${color}` }} />
          </div>
        </div>
      ))}
    </div>
  )
}
