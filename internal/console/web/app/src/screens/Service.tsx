import { useCallback, useEffect, useState } from 'react'
import { getService, restartService } from '../api'
import ScreenHeader from '../components/ScreenHeader'
import Banner from '../components/Banner'
import ConfirmDialog from '../components/ConfirmDialog'
import { useToast } from '../components/toast-context'

type StatusTone = 'ok' | 'alert' | 'warn' | 'muted'

function toneForStatus(status: string): StatusTone {
  const s = status.toLowerCase()
  if (s === 'active' || s === 'running') return 'ok'
  if (s === 'failed' || s === 'error' || s === 'dead') return 'alert'
  if (s === 'inactive' || s === 'stopped') return 'warn'
  return 'muted'
}

export default function Service() {
  const toast = useToast()
  const [status, setStatus] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [restarting, setRestarting] = useState(false)
  const [output, setOutput] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const s = await getService()
      setStatus(s.status)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'failed to read service status')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load()
  }, [load])

  const onRestart = async () => {
    setRestarting(true)
    try {
      const res = await restartService()
      setOutput(res.output || '(no output)')
      toast.push('restart command issued', 'ok')
      setConfirmOpen(false)
      // Re-read status shortly after.
      window.setTimeout(() => void load(), 1500)
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'restart failed'
      setOutput(msg)
      toast.push('restart failed', 'alert')
    } finally {
      setRestarting(false)
    }
  }

  const tone = status ? toneForStatus(status) : 'muted'
  const toneColor =
    tone === 'ok' ? 'var(--color-neon)' : tone === 'alert' ? 'var(--color-alert)' : tone === 'warn' ? 'var(--color-warn)' : 'var(--color-muted)'

  return (
    <div style={{ padding: '1.5rem 1.75rem' }}>
      <ScreenHeader
        title="Service"
        tag="pipelock daemon control"
        right={
          <>
            <button type="button" className="btn-neon" onClick={load} disabled={loading}>
              {loading ? '…' : 'refresh'}
            </button>
            <button type="button" className="btn-alert" onClick={() => setConfirmOpen(true)}>
              ⏻ restart pipelock
            </button>
          </>
        }
      />

      {error && (
        <div style={{ marginBottom: '1rem' }}>
          <Banner tone="alert" onRetry={load}>
            Cannot read service status — the controller may be unavailable. {error}
          </Banner>
        </div>
      )}

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: '1rem' }}>
        {/* Status card */}
        <div className="panel panel--neon" style={{ display: 'flex', flexDirection: 'column', gap: '0.9rem' }}>
          <div style={{ color: 'var(--color-muted)', fontSize: '0.64rem', letterSpacing: '0.18em', textTransform: 'uppercase' }}>daemon status</div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
            <span
              style={{
                width: '14px',
                height: '14px',
                borderRadius: '50%',
                background: toneColor,
                boxShadow: `0 0 12px ${toneColor}`,
                animation: tone === 'ok' ? 'pulse 2s infinite' : 'none',
              }}
            />
            <span style={{ fontSize: '1.6rem', fontWeight: 800, color: toneColor, letterSpacing: '0.05em', textShadow: `0 0 14px color-mix(in srgb, ${toneColor} 40%, transparent)` }}>
              {loading && !status ? '…' : (status ?? 'unknown').toUpperCase()}
            </span>
          </div>
          <div style={{ color: 'var(--color-muted)', fontSize: '0.72rem' }}>
            {tone === 'ok'
              ? 'pipelock is running and inspecting traffic.'
              : tone === 'alert'
                ? 'pipelock is in a failed state — restart may be required.'
                : tone === 'warn'
                  ? 'pipelock is not currently active.'
                  : 'status reported by the service controller.'}
          </div>
        </div>

        {/* Version card — backend exposes no version endpoint, so omit the value. */}
        <div className="panel" style={{ display: 'flex', flexDirection: 'column', gap: '0.9rem' }}>
          <div style={{ color: 'var(--color-muted)', fontSize: '0.64rem', letterSpacing: '0.18em', textTransform: 'uppercase' }}>build / version</div>
          <div style={{ fontSize: '1.6rem', fontWeight: 800, color: 'var(--color-muted)' }}>—</div>
          <div style={{ color: 'var(--color-muted)', fontSize: '0.72rem' }}>no version endpoint is exposed by the console API.</div>
        </div>
      </div>

      {/* Restart output */}
      {output !== null && (
        <div className="panel" style={{ marginTop: '1rem' }}>
          <div style={{ color: 'var(--color-muted)', fontSize: '0.64rem', letterSpacing: '0.18em', textTransform: 'uppercase', marginBottom: '0.5rem' }}>
            last restart output
          </div>
          <pre
            style={{
              margin: 0,
              background: 'var(--color-bg)',
              border: '1px solid var(--color-border)',
              borderRadius: 'var(--radius-panel)',
              padding: '0.85rem',
              fontSize: '0.74rem',
              color: 'var(--color-text)',
              overflow: 'auto',
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word',
            }}
          >
            {output}
          </pre>
        </div>
      )}

      <ConfirmDialog
        open={confirmOpen}
        title="Restart pipelock?"
        tone="alert"
        confirmLabel="RESTART"
        busy={restarting}
        onCancel={() => !restarting && setConfirmOpen(false)}
        onConfirm={onRestart}
        body={
          <>
            This restarts the pipelock daemon. Active tunnels and WebSocket connections will be dropped while it cycles. Proceed?
          </>
        }
      />
    </div>
  )
}
