import type { ReactNode } from 'react'

interface BannerProps {
  tone: 'alert' | 'warn' | 'info'
  children: ReactNode
  onRetry?: () => void
}

// Inline status banner — used when a backend fetch fails (e.g. pipelock down).
export default function Banner({ tone, children, onRetry }: BannerProps) {
  const color = tone === 'alert' ? 'var(--color-alert)' : tone === 'warn' ? 'var(--color-warn)' : 'var(--color-neon)'
  const borderColor = tone === 'alert' ? 'var(--color-alert-dim)' : tone === 'warn' ? '#5a5000' : 'var(--color-neon-dim)'
  return (
    <div
      role="alert"
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: '0.75rem',
        padding: '0.7rem 1rem',
        border: `1px solid ${borderColor}`,
        borderLeft: `3px solid ${color}`,
        borderRadius: 'var(--radius-panel)',
        background: 'color-mix(in srgb, ' + color + ' 6%, var(--color-surface))',
        color: 'var(--color-text)',
        fontSize: '0.8rem',
      }}
    >
      <span style={{ color }} aria-hidden>
        {tone === 'alert' ? '⚠' : tone === 'warn' ? '▲' : 'ℹ'}
      </span>
      <span style={{ flex: 1 }}>{children}</span>
      {onRetry && (
        <button type="button" className="btn-neon" style={{ padding: '0.25rem 0.7rem', fontSize: '0.7rem' }} onClick={onRetry}>
          retry
        </button>
      )}
    </div>
  )
}
