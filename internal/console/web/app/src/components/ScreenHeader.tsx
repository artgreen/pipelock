import type { ReactNode } from 'react'

interface ScreenHeaderProps {
  title: string
  tag: string
  right?: ReactNode
  children?: ReactNode
}

// Consistent page header: title + section tag on the left, controls on the right.
export default function ScreenHeader({ title, tag, right, children }: ScreenHeaderProps) {
  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'flex-end',
        justifyContent: 'space-between',
        gap: '1rem',
        marginBottom: '1.25rem',
        flexWrap: 'wrap',
      }}
    >
      <div>
        <div style={{ color: 'var(--color-muted)', fontSize: '0.62rem', letterSpacing: '0.3em', textTransform: 'uppercase' }}>{tag}</div>
        <h1 className="glow-neon" style={{ margin: '0.2rem 0 0', fontSize: '1.4rem', fontWeight: 800, letterSpacing: '0.06em' }}>
          {title}
        </h1>
        {children}
      </div>
      {right && <div style={{ display: 'flex', gap: '0.6rem', alignItems: 'center', flexWrap: 'wrap' }}>{right}</div>}
    </div>
  )
}
