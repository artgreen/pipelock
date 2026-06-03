import { useEffect, type ReactNode } from 'react'

interface DrawerProps {
  open: boolean
  title: string
  subtitle?: string
  onClose: () => void
  children: ReactNode
}

// Right-side sliding detail drawer with backdrop. Escape closes.
export default function Drawer({ open, title, subtitle, onClose, children }: DrawerProps) {
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, onClose])

  return (
    <div
      aria-hidden={!open}
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 200,
        pointerEvents: open ? 'auto' : 'none',
      }}
    >
      {/* Backdrop */}
      <div
        onClick={onClose}
        style={{
          position: 'absolute',
          inset: 0,
          background: 'rgba(0,0,0,0.6)',
          opacity: open ? 1 : 0,
          transition: 'opacity 0.2s ease',
        }}
      />
      {/* Panel */}
      <aside
        className="scanlines"
        style={{
          position: 'absolute',
          top: 0,
          right: 0,
          height: '100%',
          width: 'min(560px, 92vw)',
          background: 'var(--color-surface)',
          borderLeft: '1px solid var(--color-neon-dim)',
          boxShadow: '-12px 0 40px rgba(0,0,0,0.5)',
          transform: open ? 'translateX(0)' : 'translateX(100%)',
          transition: 'transform 0.22s cubic-bezier(0.2, 0.8, 0.2, 1)',
          display: 'flex',
          flexDirection: 'column',
        }}
      >
        <header
          style={{
            display: 'flex',
            alignItems: 'flex-start',
            justifyContent: 'space-between',
            padding: '1rem 1.25rem',
            borderBottom: '1px solid var(--color-border)',
            flexShrink: 0,
          }}
        >
          <div>
            <div className="glow-neon" style={{ fontSize: '0.85rem', letterSpacing: '0.12em', textTransform: 'uppercase', fontWeight: 700 }}>
              {title}
            </div>
            {subtitle && (
              <div style={{ color: 'var(--color-muted)', fontSize: '0.72rem', marginTop: '0.25rem', wordBreak: 'break-all' }}>
                {subtitle}
              </div>
            )}
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="close"
            style={{
              background: 'transparent',
              border: '1px solid var(--color-border)',
              color: 'var(--color-muted)',
              cursor: 'pointer',
              padding: '0.15rem 0.5rem',
              fontFamily: 'var(--font-mono)',
              borderRadius: 'var(--radius-panel)',
            }}
          >
            ✕
          </button>
        </header>
        <div style={{ flex: 1, overflow: 'auto', padding: '1.25rem' }}>{children}</div>
      </aside>
    </div>
  )
}

// Pretty-printed JSON block for drawer bodies.
export function JsonBlock({ value }: { value: unknown }) {
  return (
    <pre
      style={{
        background: 'var(--color-bg)',
        border: '1px solid var(--color-border)',
        borderRadius: 'var(--radius-panel)',
        padding: '0.85rem',
        fontSize: '0.74rem',
        lineHeight: 1.65,
        color: 'var(--color-text)',
        overflow: 'auto',
        whiteSpace: 'pre-wrap',
        wordBreak: 'break-word',
        margin: 0,
      }}
    >
      {JSON.stringify(value, null, 2)}
    </pre>
  )
}
