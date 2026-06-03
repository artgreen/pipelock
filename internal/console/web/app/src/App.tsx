export default function App() {
  return (
    <div
      className="scanlines"
      style={{
        minHeight: '100vh',
        display: 'flex',
        flexDirection: 'column',
        backgroundColor: 'var(--color-bg)',
      }}
    >
      {/* Top bar */}
      <header
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '0 1.5rem',
          height: '48px',
          borderBottom: '1px solid var(--color-border)',
          backgroundColor: 'var(--color-surface)',
          flexShrink: 0,
        }}
      >
        <span className="glow-neon" style={{ fontSize: '0.95rem', letterSpacing: '0.15em', textTransform: 'uppercase', fontWeight: 700 }}>
          &#9632; pipelock console
        </span>
        <span className="badge badge--ok">agent firewall</span>
      </header>

      {/* Main content */}
      <main
        style={{
          flex: 1,
          padding: '2rem 1.5rem',
          display: 'grid',
          gap: '1rem',
          gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
          alignContent: 'start',
        }}
      >
        {/* Status panel */}
        <div className="panel panel--neon">
          <div style={{ marginBottom: '0.5rem', color: 'var(--color-muted)', fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase' }}>
            proxy status
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
            <span className="badge badge--ok">active</span>
            <span style={{ color: 'var(--color-text)', fontSize: '0.85rem' }}>0 sessions</span>
          </div>
          <hr className="divider" />
          <div style={{ color: 'var(--color-muted)', fontSize: '0.75rem' }}>
            frontend build verified &#10003;
          </div>
        </div>

        {/* Kill switch panel */}
        <div className="panel">
          <div style={{ marginBottom: '0.5rem', color: 'var(--color-muted)', fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase' }}>
            kill switch
          </div>
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <button type="button" className="btn-alert">Engage</button>
            <button type="button" className="btn-neon">Status</button>
          </div>
        </div>

        {/* Placeholder — screens next task */}
        <div className="panel">
          <div style={{ color: 'var(--color-muted)', fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase', marginBottom: '0.5rem' }}>
            event stream
          </div>
          <div style={{ color: 'var(--color-muted)', fontSize: '0.8rem' }}>
            — no events —
          </div>
        </div>
      </main>

      {/* Footer */}
      <footer
        style={{
          padding: '0.5rem 1.5rem',
          borderTop: '1px solid var(--color-border)',
          color: 'var(--color-muted)',
          fontSize: '0.7rem',
          letterSpacing: '0.05em',
          flexShrink: 0,
        }}
      >
        pipelock &mdash; agent firewall runtime
      </footer>
    </div>
  )
}
