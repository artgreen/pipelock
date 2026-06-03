import { useEffect, type ReactNode } from 'react'

interface ConfirmDialogProps {
  open: boolean
  title: string
  body: ReactNode
  confirmLabel: string
  tone?: 'neon' | 'alert'
  busy?: boolean
  onConfirm: () => void
  onCancel: () => void
}

// Modal confirm dialog. Escape cancels. Used for kill switch + restart.
export default function ConfirmDialog({
  open,
  title,
  body,
  confirmLabel,
  tone = 'neon',
  busy = false,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !busy) onCancel()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, busy, onCancel])

  if (!open) return null

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 300,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: 'rgba(0,0,0,0.7)',
      }}
      onClick={() => !busy && onCancel()}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        className={tone === 'alert' ? 'panel' : 'panel panel--neon'}
        style={{
          width: 'min(440px, 92vw)',
          borderColor: tone === 'alert' ? 'var(--color-alert)' : 'var(--color-neon-dim)',
          boxShadow:
            tone === 'alert'
              ? '0 0 24px color-mix(in srgb, var(--color-alert) 30%, transparent)'
              : '0 0 18px color-mix(in srgb, var(--color-neon) 25%, transparent)',
          padding: '1.25rem',
        }}
      >
        <div
          className={tone === 'alert' ? 'glow-alert' : 'glow-neon'}
          style={{ fontSize: '0.9rem', letterSpacing: '0.1em', textTransform: 'uppercase', fontWeight: 700, marginBottom: '0.75rem' }}
        >
          {title}
        </div>
        <div style={{ color: 'var(--color-text)', fontSize: '0.82rem', lineHeight: 1.7, marginBottom: '1.25rem' }}>{body}</div>
        <div style={{ display: 'flex', gap: '0.6rem', justifyContent: 'flex-end' }}>
          <button type="button" className="btn-neon" onClick={onCancel} disabled={busy} style={{ opacity: busy ? 0.5 : 1 }}>
            Cancel
          </button>
          <button
            type="button"
            className={tone === 'alert' ? 'btn-alert' : 'btn-neon'}
            onClick={onConfirm}
            disabled={busy}
            style={{ opacity: busy ? 0.6 : 1 }}
          >
            {busy ? '…working' : confirmLabel}
          </button>
        </div>
      </div>
    </div>
  )
}
