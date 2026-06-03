import { useState } from 'react'
import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { logout } from '../api'
import { useKillSwitch } from './killswitch-context'
import ConfirmDialog from './ConfirmDialog'
import { useToast } from './toast-context'

const NAV = [
  { to: '/', label: 'Overview', glyph: '◉', end: true },
  { to: '/events', label: 'Events', glyph: '⌁', end: false },
  { to: '/sessions', label: 'Sessions', glyph: '⊞', end: false },
  { to: '/config', label: 'Config', glyph: '⚙', end: false },
  { to: '/service', label: 'Service', glyph: '⏻', end: false },
]

// The persistent app chrome: left rail + top status bar with kill switch.
export default function Layout() {
  const { data: ks, toggle } = useKillSwitch()
  const toast = useToast()
  const navigate = useNavigate()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [busy, setBusy] = useState(false)

  const active = ks?.active ?? false
  const desired = !active

  const onConfirm = async () => {
    setBusy(true)
    try {
      await toggle(desired)
      toast.push(desired ? 'KILL SWITCH ENGAGED — all traffic denied' : 'Kill switch released — traffic resumed', desired ? 'alert' : 'ok')
      setConfirmOpen(false)
    } catch (e) {
      toast.push(e instanceof Error ? e.message : 'kill switch toggle failed', 'alert')
    } finally {
      setBusy(false)
    }
  }

  const onLogout = async () => {
    try {
      await logout()
    } catch {
      /* ignore — navigate regardless */
    }
    navigate('/login')
  }

  return (
    <div className="scanlines" style={{ height: '100vh', display: 'flex', overflow: 'hidden', background: 'var(--color-bg)' }}>
      {/* ─── Sidebar ─── */}
      <nav
        style={{
          width: '208px',
          flexShrink: 0,
          background: 'var(--color-surface)',
          borderRight: '1px solid var(--color-border)',
          display: 'flex',
          flexDirection: 'column',
          padding: '1.1rem 0',
        }}
      >
        <div style={{ padding: '0 1.1rem 1.1rem', borderBottom: '1px solid var(--color-border)', marginBottom: '0.75rem' }}>
          <div className="glow-neon" style={{ fontSize: '1rem', fontWeight: 800, letterSpacing: '0.12em', display: 'flex', alignItems: 'center', gap: '0.45rem' }}>
            <span style={{ fontSize: '0.8rem' }}>▮▮</span> PIPELOCK
          </div>
          <div style={{ color: 'var(--color-muted)', fontSize: '0.62rem', letterSpacing: '0.28em', textTransform: 'uppercase', marginTop: '0.3rem' }}>
            egress monitor
          </div>
        </div>

        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: '0.15rem', padding: '0 0.6rem' }}>
          {NAV.map((item) => (
            <NavLink key={item.to} to={item.to} end={item.end} style={navLinkStyle}>
              {({ isActive }) => (
                <span style={navItemInner(isActive)}>
                  <span style={{ width: '1.1rem', textAlign: 'center', opacity: isActive ? 1 : 0.6 }}>{item.glyph}</span>
                  <span>{item.label}</span>
                  {isActive && <span style={{ marginLeft: 'auto', color: 'var(--color-neon)' }}>▸</span>}
                </span>
              )}
            </NavLink>
          ))}
        </div>

        <div style={{ padding: '0.75rem 1.1rem 0', borderTop: '1px solid var(--color-border)' }}>
          <button
            type="button"
            onClick={onLogout}
            style={{
              background: 'transparent',
              border: 'none',
              color: 'var(--color-muted)',
              cursor: 'pointer',
              fontFamily: 'var(--font-mono)',
              fontSize: '0.72rem',
              letterSpacing: '0.08em',
              textTransform: 'uppercase',
              padding: 0,
            }}
          >
            ⎋ sign out
          </button>
        </div>
      </nav>

      {/* ─── Main column ─── */}
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0 }}>
        {/* Top status bar */}
        <header
          style={{
            height: '54px',
            flexShrink: 0,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '0 1.25rem',
            borderBottom: '1px solid var(--color-border)',
            background: active ? 'color-mix(in srgb, var(--color-alert) 8%, var(--color-surface))' : 'var(--color-surface)',
            transition: 'background 0.3s',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
            <span style={{ display: 'flex', alignItems: 'center', gap: '0.45rem' }}>
              <span
                style={{
                  width: '8px',
                  height: '8px',
                  borderRadius: '50%',
                  background: active ? 'var(--color-alert)' : 'var(--color-neon)',
                  boxShadow: `0 0 8px ${active ? 'var(--color-alert)' : 'var(--color-neon)'}`,
                  animation: 'pulse 2s infinite',
                }}
              />
              <span style={{ fontSize: '0.74rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: active ? 'var(--color-alert)' : 'var(--color-neon)' }}>
                {active ? 'traffic blocked' : 'monitoring'}
              </span>
            </span>
            {ks && ks.message && (
              <span style={{ color: 'var(--color-muted)', fontSize: '0.72rem' }}>{ks.message}</span>
            )}
          </div>

          {/* Kill switch toggle */}
          <button
            type="button"
            onClick={() => setConfirmOpen(true)}
            className={active ? 'btn-neon' : 'btn-alert'}
            style={{
              fontWeight: 700,
              letterSpacing: '0.12em',
              ...(active
                ? {}
                : { boxShadow: '0 0 10px color-mix(in srgb, var(--color-alert) 35%, transparent)' }),
            }}
          >
            <span style={{ fontSize: '0.9rem' }}>⏻</span>
            {active ? 'RELEASE KILL SWITCH' : 'KILL SWITCH'}
          </button>
        </header>

        {/* Routed screen */}
        <main style={{ flex: 1, overflow: 'auto', minHeight: 0 }}>
          <Outlet />
        </main>
      </div>

      <ConfirmDialog
        open={confirmOpen}
        title={desired ? 'Engage kill switch?' : 'Release kill switch?'}
        tone={desired ? 'alert' : 'neon'}
        confirmLabel={desired ? 'ENGAGE' : 'RELEASE'}
        busy={busy}
        onCancel={() => !busy && setConfirmOpen(false)}
        onConfirm={onConfirm}
        body={
          desired ? (
            <>
              This immediately denies <strong>all</strong> agent egress traffic until released.
              In-flight requests are cut. Confirm only if you intend a hard stop.
            </>
          ) : (
            <>Releasing the config-sourced kill switch resumes traffic. Other active sources (API, signal, sentinel file) remain independent and may keep traffic blocked.</>
          )
        }
      />

      <style>{`@keyframes pulse { 0%,100% { opacity: 1; } 50% { opacity: 0.4; } }`}</style>
    </div>
  )
}

const navLinkStyle = { textDecoration: 'none' } as const

function navItemInner(active: boolean): React.CSSProperties {
  return {
    display: 'flex',
    alignItems: 'center',
    gap: '0.6rem',
    padding: '0.5rem 0.65rem',
    borderRadius: 'var(--radius-panel)',
    fontSize: '0.82rem',
    letterSpacing: '0.04em',
    color: active ? 'var(--color-neon)' : 'var(--color-text)',
    background: active ? 'color-mix(in srgb, var(--color-neon) 10%, transparent)' : 'transparent',
    borderLeft: active ? '2px solid var(--color-neon)' : '2px solid transparent',
    transition: 'background 0.15s, color 0.15s',
    cursor: 'pointer',
  }
}
