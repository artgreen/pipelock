import type { ReactNode } from 'react'

interface AuthShellProps {
  heading: string
  sub: string
  children: ReactNode
}

// Centered terminal-card frame shared by Login and Setup.
export default function AuthShell({ heading, sub, children }: AuthShellProps) {
  return (
    <div
      className="scanlines"
      style={{
        height: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background:
          'radial-gradient(ellipse at 50% 0%, color-mix(in srgb, var(--color-neon) 6%, transparent), transparent 60%), var(--color-bg)',
      }}
    >
      <div
        className="panel panel--neon"
        style={{ width: 'min(420px, 92vw)', padding: '2rem 1.75rem', boxShadow: '0 0 40px color-mix(in srgb, var(--color-neon) 12%, transparent)' }}
      >
        <div className="glow-neon" style={{ fontSize: '1.3rem', fontWeight: 800, letterSpacing: '0.18em', textAlign: 'center' }}>
          ▮▮ PIPELOCK
        </div>
        <div style={{ textAlign: 'center', color: 'var(--color-muted)', fontSize: '0.62rem', letterSpacing: '0.32em', textTransform: 'uppercase', marginTop: '0.35rem' }}>
          console access
        </div>
        <hr className="divider" style={{ margin: '1.25rem 0' }} />
        <h1 style={{ fontSize: '0.95rem', margin: 0, color: 'var(--color-text)', letterSpacing: '0.05em' }}>{heading}</h1>
        <p style={{ color: 'var(--color-muted)', fontSize: '0.74rem', marginTop: '0.35rem', marginBottom: '1.25rem', lineHeight: 1.6 }}>{sub}</p>
        {children}
      </div>
    </div>
  )
}
