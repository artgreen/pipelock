import { useEffect, useState } from 'react'
import { ApiError, proposeUnblock, type UnblockProposal } from '../../api'
import { addToSequence, removeFromSequence } from '../../lib/yamlpatch'

const REASONS = ['ssrf_private_ip', 'ssrf_metadata', 'domain_blocklist'] as const

interface Props {
  target: string
  reason?: string
  matchedPattern?: string // blocklist pattern that matched, if known (from event)
  buffer: string
  onCancel: () => void
  // Applies the patched buffer through the parent's validate->apply path.
  onApply: (patched: string, summary: string) => Promise<void>
}

export default function UnblockDialog({ target, reason, matchedPattern, buffer, onCancel, onApply }: Props) {
  const [tgt, setTgt] = useState(target)
  const [rsn, setRsn] = useState(reason ?? '')
  const [prop, setProp] = useState<UnblockProposal | null>(null)
  const [err, setErr] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [applying, setApplying] = useState(false)

  const fetchProposal = async () => {
    setLoading(true)
    setErr(null)
    setProp(null)
    try {
      setProp(await proposeUnblock(tgt.trim(), rsn, matchedPattern ?? ''))
    } catch (e) {
      setErr(e instanceof ApiError ? e.body || e.message : e instanceof Error ? e.message : 'failed')
    } finally {
      setLoading(false)
    }
  }

  // Auto-fetch when we arrive with both fields prefilled (event-driven path).
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (target.trim() && reason) void fetchProposal()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const confirm = async () => {
    if (!prop) return
    const patched = prop.op === 'list_add'
      ? addToSequence(buffer, prop.path, prop.value)
      : removeFromSequence(buffer, prop.path, prop.value)
    setApplying(true)
    try {
      await onApply(patched, `${prop.op === 'list_add' ? 'allow' : 'unblock'} ${prop.value}`)
    } finally {
      setApplying(false)
    }
  }

  return (
    <div style={overlay}>
      <div className="panel panel--neon" style={modal}>
        <h3 style={{ marginTop: 0, color: 'var(--color-neon)' }}>Allow a blocked destination</h3>

        <label style={lbl}>destination</label>
        <input value={tgt} onChange={(e) => setTgt(e.target.value)} style={field} placeholder="host, IP, or URL" />

        <label style={lbl}>block reason</label>
        <select value={rsn} onChange={(e) => setRsn(e.target.value)} style={field}>
          <option value="">— select —</option>
          {REASONS.map((r) => <option key={r} value={r}>{r}</option>)}
        </select>

        <div style={{ margin: '0.7rem 0' }}>
          <button type="button" className="btn-neon" disabled={!tgt.trim() || !rsn || loading} onClick={fetchProposal}>
            {loading ? '…checking' : 'preview change'}
          </button>
        </div>

        {err && <div style={{ color: 'var(--color-alert)', fontSize: '0.74rem', marginBottom: '0.6rem' }}>✕ {err}</div>}

        {prop && (
          <div className="panel" style={{ marginBottom: '0.7rem' }}>
            <p style={{ marginTop: 0, fontSize: '0.78rem' }}>{prop.explanation}</p>
            <div style={{ fontSize: '0.72rem', color: 'var(--color-muted)' }}>
              <strong>still scanned:</strong>
              <ul style={{ margin: '0.3rem 0 0', paddingLeft: '1.1rem' }}>
                {prop.still_scanned.map((s) => <li key={s}>{s}</li>)}
              </ul>
            </div>
            <pre style={diffLine}>+ {prop.path}: {prop.op === 'list_add' ? `add ${prop.value}` : `remove ${prop.value}`}</pre>
            {prop.warning && <div style={warnBox}>⚠ {prop.warning}</div>}
          </div>
        )}

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.5rem' }}>
          <button type="button" className="btn-neon" onClick={onCancel} disabled={applying}>cancel</button>
          <button type="button" className="btn-alert" onClick={confirm} disabled={!prop || applying}>
            {applying ? '…applying' : 'confirm & apply'}
          </button>
        </div>
      </div>
    </div>
  )
}

const overlay: React.CSSProperties = { position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.6)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 50 }
const modal: React.CSSProperties = { width: 'min(560px, 92vw)', maxHeight: '88vh', overflow: 'auto', padding: '1.25rem' }
const lbl: React.CSSProperties = { display: 'block', fontSize: '0.62rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'var(--color-muted)', margin: '0.5rem 0 0.2rem' }
const field: React.CSSProperties = { width: '100%', background: 'var(--color-bg)', border: '1px solid var(--color-border)', color: 'var(--color-text)', padding: '0.4rem 0.5rem', fontFamily: 'var(--font-mono)', fontSize: '0.78rem' }
const diffLine: React.CSSProperties = { background: 'rgba(57,255,20,0.08)', color: 'var(--color-neon)', padding: '0.4rem 0.6rem', fontSize: '0.74rem', margin: '0.5rem 0 0', whiteSpace: 'pre-wrap' }
const warnBox: React.CSSProperties = { marginTop: '0.5rem', border: '1px solid #5a5000', background: 'rgba(255,200,0,0.08)', color: 'var(--color-warn)', padding: '0.5rem 0.6rem', fontSize: '0.74rem' }
