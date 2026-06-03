import { useState } from 'react'
import { addToSequence, readSequence, removeFromSequence } from '../../lib/yamlpatch'

interface Props {
  label: string
  help: string
  path: string
  buffer: string
  disabled?: boolean
  onChange: (nextBuffer: string) => void
}

export default function ListEditor({ label, help, path, buffer, disabled, onChange }: Props) {
  const [draft, setDraft] = useState('')
  const items = readSequence(buffer, path)

  const add = () => {
    const v = draft.trim()
    if (!v) return
    onChange(addToSequence(buffer, path, v))
    setDraft('')
  }

  return (
    <div className="panel" style={{ marginBottom: '0.9rem' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', gap: '1rem' }}>
        <span style={{ fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'var(--color-text)' }}>{label}</span>
        <code style={{ color: 'var(--color-muted)', fontSize: '0.62rem' }}>{path}</code>
      </div>
      <p style={{ color: 'var(--color-muted)', fontSize: '0.72rem', margin: '0.35rem 0 0.6rem' }}>{help}</p>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.4rem', marginBottom: '0.6rem' }}>
        {items.length === 0 && <span style={{ color: 'var(--color-muted)', fontSize: '0.72rem' }}>— empty —</span>}
        {items.map((it) => (
          <span key={it} style={chip}>
            {it}
            <button type="button" disabled={disabled} onClick={() => onChange(removeFromSequence(buffer, path, it))} style={chipX} aria-label={`remove ${it}`}>×</button>
          </span>
        ))}
      </div>
      <div style={{ display: 'flex', gap: '0.4rem' }}>
        <input
          value={draft}
          disabled={disabled}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && add()}
          placeholder="add an entry…"
          style={{ flex: 1, background: 'var(--color-bg)', border: '1px solid var(--color-border)', color: 'var(--color-text)', padding: '0.35rem 0.5rem', fontFamily: 'var(--font-mono)', fontSize: '0.74rem' }}
        />
        <button type="button" className="btn-neon" disabled={disabled || !draft.trim()} onClick={add}>add</button>
      </div>
    </div>
  )
}

const chip: React.CSSProperties = {
  display: 'inline-flex', alignItems: 'center', gap: '0.4rem',
  border: '1px solid var(--color-border)', borderRadius: 'var(--radius-panel)',
  padding: '0.2rem 0.5rem', fontFamily: 'var(--font-mono)', fontSize: '0.72rem', color: 'var(--color-text)',
}
const chipX: React.CSSProperties = {
  background: 'none', border: 'none', color: 'var(--color-alert)', cursor: 'pointer', fontSize: '0.9rem', lineHeight: 1, padding: 0,
}
