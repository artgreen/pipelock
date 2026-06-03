import { useState } from 'react'
import { REDACTED_SENTINEL, type SchemaField } from '../../api'
import { coerce } from './fieldvalue'

interface Props {
  field: SchemaField
  value: unknown          // current effective value (may be REDACTED_SENTINEL for secrets)
  present: boolean        // explicitly set in the file (vs default)
  onChange: (path: string, value: unknown) => void  // value null => reset to default
  onAdvanced?: () => void // switch to raw editor (for opaque/advanced fields)
}

export default function Field({ field, value, present, onChange, onAdvanced }: Props) {
  return (
    <div style={wrapStyle}>
      {field.type === 'group'
        ? <GroupWidget field={field} value={value} present={present} onChange={onChange} onAdvanced={onAdvanced} />
        : (
          <>
            <FieldHeader field={field} present={present} />
            <FieldWidget field={field} value={value} present={present} onChange={onChange} onAdvanced={onAdvanced} />
          </>
        )}
    </div>
  )
}

// ─── Header (label + path + help + overridden badge) ────────────────────────

function FieldHeader({ field, present }: { field: SchemaField; present: boolean }) {
  return (
    <div style={{ marginBottom: '0.4rem' }}>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: '0.5rem', flexWrap: 'wrap' }}>
        <span style={labelStyle}>{field.label}</span>
        {present && <span style={overriddenBadge}>overridden</span>}
        <code style={pathStyle}>{field.path}</code>
      </div>
      {field.help && (
        <p style={helpStyle}>{field.help}</p>
      )}
      {field.default !== undefined && field.default !== null && (
        <p style={defaultStyle}>default: <code style={{ fontFamily: 'var(--font-mono)' }}>{String(field.default)}</code></p>
      )}
    </div>
  )
}

// ─── Dispatcher ─────────────────────────────────────────────────────────────

function FieldWidget({ field, value, present: _present, onChange, onAdvanced }: Props) {
  switch (field.type) {
    case 'group':
      return <GroupWidget field={field} value={value} present={_present} onChange={onChange} onAdvanced={onAdvanced} />
    case 'bool':
      return <BoolWidget field={field} value={value} onChange={onChange} />
    case 'tristate':
      return <TristateWidget field={field} value={value} onChange={onChange} />
    case 'enum':
      return <EnumWidget field={field} value={value} onChange={onChange} />
    case 'int':
    case 'float':
      return <NumberWidget field={field} value={value} onChange={onChange} />
    case 'string':
      return <StringWidget field={field} value={value} onChange={onChange} />
    case 'list':
      return <ListWidget field={field} value={value} onChange={onChange} />
    case 'map':
      return <MapWidget field={field} value={value} onChange={onChange} />
    case 'opaque':
      return <OpaqueWidget onAdvanced={onAdvanced} />
    default:
      return null
  }
}

// ─── Group widget ────────────────────────────────────────────────────────────

function GroupWidget({ field, value, present, onChange, onAdvanced }: Props) {
  const groupObj = (value != null && typeof value === 'object' && !Array.isArray(value))
    ? (value as Record<string, unknown>)
    : {}

  return (
    <div>
      <div style={subheadingStyle}>{field.label}</div>
      {field.children && field.children.length > 0 && (
        <div style={{ paddingLeft: '0.75rem', borderLeft: '2px solid var(--color-border)' }}>
          {field.children.map((child) => (
            <Field
              key={child.path}
              field={child}
              value={groupObj[child.key]}
              present={present}
              onChange={onChange}
              onAdvanced={onAdvanced}
            />
          ))}
        </div>
      )}
    </div>
  )
}

// ─── Bool widget ─────────────────────────────────────────────────────────────

function BoolWidget({ field, value, onChange }: { field: SchemaField; value: unknown; onChange: Props['onChange'] }) {
  const checked = value === true
  return (
    <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer' }}>
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(field.path, e.target.checked)}
        style={{ accentColor: 'var(--color-neon)', width: '1rem', height: '1rem', cursor: 'pointer' }}
      />
      <span style={{ fontSize: '0.78rem', color: checked ? 'var(--color-neon)' : 'var(--color-muted)' }}>
        {checked ? 'on' : 'off'}
      </span>
    </label>
  )
}

// ─── Tristate widget ─────────────────────────────────────────────────────────

function TristateWidget({ field, value, onChange }: { field: SchemaField; value: unknown; onChange: Props['onChange'] }) {
  const active = value === true ? 'on' : value === false ? 'off' : 'default'

  return (
    <div style={{ display: 'flex', gap: '0.25rem' }}>
      <button
        type="button"
        onClick={() => onChange(field.path, null)}
        style={tristateChip(active === 'default')}
      >
        Default{field.default !== undefined ? ` (${String(field.default)})` : ''}
      </button>
      <button
        type="button"
        onClick={() => onChange(field.path, true)}
        style={tristateChip(active === 'on')}
      >
        On
      </button>
      <button
        type="button"
        onClick={() => onChange(field.path, false)}
        style={tristateChip(active === 'off')}
      >
        Off
      </button>
    </div>
  )
}

// ─── Enum widget ─────────────────────────────────────────────────────────────

function EnumWidget({ field, value, onChange }: { field: SchemaField; value: unknown; onChange: Props['onChange'] }) {
  const options = field.enum ?? []
  const strVal = value != null ? String(value) : ''
  // Include current value if not in the declared enum list
  const allOptions = strVal && !options.includes(strVal) ? [strVal, ...options] : options

  return (
    <select
      value={strVal}
      onChange={(e) => onChange(field.path, e.target.value)}
      style={selectStyle}
    >
      {!strVal && <option value="">— select —</option>}
      {allOptions.map((opt) => (
        <option key={opt} value={opt}>{opt}</option>
      ))}
    </select>
  )
}

// ─── Number widget ────────────────────────────────────────────────────────────

function NumberWidget({ field, value, onChange }: { field: SchemaField; value: unknown; onChange: Props['onChange'] }) {
  const strVal = value != null ? String(value) : ''
  return (
    <input
      type="number"
      value={strVal}
      step={field.type === 'float' ? 'any' : '1'}
      onChange={(e) => onChange(field.path, coerce(field.type, e.target.value))}
      style={inputStyle}
    />
  )
}

// ─── String widget ────────────────────────────────────────────────────────────

function StringWidget({ field, value, onChange }: { field: SchemaField; value: unknown; onChange: Props['onChange'] }) {
  // For secrets: only emit onChange once the user starts typing.
  // REDACTED_SENTINEL means "leave unchanged" — show placeholder, not the sentinel.
  const isRedacted = value === REDACTED_SENTINEL

  if (field.secret) {
    return <SecretInput field={field} isRedacted={isRedacted} value={isRedacted ? '' : (value != null ? String(value) : '')} onChange={onChange} />
  }

  return (
    <input
      type="text"
      value={value != null ? String(value) : ''}
      onChange={(e) => onChange(field.path, e.target.value)}
      style={inputStyle}
    />
  )
}

function SecretInput({ field, isRedacted, value, onChange }: { field: SchemaField; isRedacted: boolean; value: string; onChange: Props['onChange'] }) {
  // Track whether user has started editing. Until they type, we don't emit.
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState('')

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!editing) setEditing(true)
    setDraft(e.target.value)
    onChange(field.path, e.target.value)
  }

  return (
    <input
      type="password"
      value={editing ? draft : value}
      placeholder={isRedacted ? '•• set ••' : ''}
      onChange={handleChange}
      style={inputStyle}
    />
  )
}

// ─── List (chips) widget ──────────────────────────────────────────────────────

function ListWidget({ field, value, onChange }: { field: SchemaField; value: unknown; onChange: Props['onChange'] }) {
  const [draft, setDraft] = useState('')
  const items: string[] = Array.isArray(value) ? (value as string[]) : []

  const add = () => {
    const v = draft.trim()
    if (!v) return
    onChange(field.path, [...items, v])
    setDraft('')
  }

  const remove = (item: string) => {
    onChange(field.path, items.filter((it) => it !== item))
  }

  return (
    <div>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.4rem', marginBottom: '0.5rem' }}>
        {items.length === 0 && (
          <span style={{ color: 'var(--color-muted)', fontSize: '0.72rem' }}>— empty —</span>
        )}
        {items.map((it) => (
          <span key={it} style={chipStyle}>
            {it}
            <button
              type="button"
              onClick={() => remove(it)}
              style={chipXStyle}
              aria-label={`remove ${it}`}
            >
              ×
            </button>
          </span>
        ))}
      </div>
      <div style={{ display: 'flex', gap: '0.4rem' }}>
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && add()}
          placeholder="add an entry…"
          style={{ ...inputStyle, flex: 1 }}
        />
        <button
          type="button"
          className="btn-neon"
          disabled={!draft.trim()}
          onClick={add}
        >
          add
        </button>
      </div>
    </div>
  )
}

// ─── Map (key/value rows) widget ──────────────────────────────────────────────

function MapWidget({ field, value, onChange }: { field: SchemaField; value: unknown; onChange: Props['onChange'] }) {
  const [draftKey, setDraftKey] = useState('')
  const [draftVal, setDraftVal] = useState('')
  const obj: Record<string, string> =
    value != null && typeof value === 'object' && !Array.isArray(value)
      ? (value as Record<string, string>)
      : {}
  const entries = Object.entries(obj)

  const addEntry = () => {
    const k = draftKey.trim()
    const v = draftVal.trim()
    if (!k) return
    onChange(field.path, { ...obj, [k]: v })
    setDraftKey('')
    setDraftVal('')
  }

  const removeEntry = (key: string) => {
    const next = { ...obj }
    delete next[key]
    onChange(field.path, next)
  }

  const updateVal = (key: string, val: string) => {
    onChange(field.path, { ...obj, [key]: val })
  }

  return (
    <div>
      {entries.length === 0 && (
        <span style={{ color: 'var(--color-muted)', fontSize: '0.72rem', display: 'block', marginBottom: '0.4rem' }}>— empty —</span>
      )}
      {entries.map(([k, v]) => (
        <div key={k} style={{ display: 'flex', gap: '0.4rem', marginBottom: '0.35rem', alignItems: 'center' }}>
          <code style={{ color: 'var(--color-neon)', fontFamily: 'var(--font-mono)', fontSize: '0.72rem', minWidth: '6rem' }}>{k}</code>
          <input
            type="text"
            value={v}
            onChange={(e) => updateVal(k, e.target.value)}
            style={{ ...inputStyle, flex: 1 }}
          />
          <button
            type="button"
            onClick={() => removeEntry(k)}
            style={{ background: 'none', border: 'none', color: 'var(--color-alert)', cursor: 'pointer', fontSize: '1rem', lineHeight: 1, padding: '0 0.25rem' }}
            aria-label={`remove ${k}`}
          >
            ×
          </button>
        </div>
      ))}
      <div style={{ display: 'flex', gap: '0.4rem', marginTop: '0.4rem' }}>
        <input
          type="text"
          value={draftKey}
          onChange={(e) => setDraftKey(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && addEntry()}
          placeholder="key"
          style={{ ...inputStyle, flex: '0 0 8rem' }}
        />
        <input
          type="text"
          value={draftVal}
          onChange={(e) => setDraftVal(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && addEntry()}
          placeholder="value"
          style={{ ...inputStyle, flex: 1 }}
        />
        <button
          type="button"
          className="btn-neon"
          disabled={!draftKey.trim()}
          onClick={addEntry}
        >
          add
        </button>
      </div>
    </div>
  )
}

// ─── Opaque widget ────────────────────────────────────────────────────────────

function OpaqueWidget({ onAdvanced }: { onAdvanced?: () => void }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', flexWrap: 'wrap' }}>
      <span style={{ color: 'var(--color-muted)', fontSize: '0.74rem' }}>
        Complex field — edit in the Advanced (raw YAML) view
      </span>
      {onAdvanced && (
        <button type="button" className="btn-neon" onClick={onAdvanced} style={{ fontSize: '0.7rem', padding: '0.25rem 0.6rem' }}>
          open advanced
        </button>
      )}
    </div>
  )
}

// ─── Shared styles ────────────────────────────────────────────────────────────

const wrapStyle: React.CSSProperties = {
  marginBottom: '0.9rem',
}

const labelStyle: React.CSSProperties = {
  fontSize: '0.7rem',
  letterSpacing: '0.1em',
  textTransform: 'uppercase',
  color: 'var(--color-text)',
  fontFamily: 'var(--font-mono)',
}

const overriddenBadge: React.CSSProperties = {
  display: 'inline-block',
  fontSize: '0.6rem',
  letterSpacing: '0.08em',
  textTransform: 'uppercase',
  color: 'var(--color-neon)',
  border: '1px solid var(--color-neon-dim)',
  borderRadius: 'var(--radius-panel)',
  padding: '0.05rem 0.35rem',
}

const pathStyle: React.CSSProperties = {
  color: 'var(--color-muted)',
  fontSize: '0.62rem',
  fontFamily: 'var(--font-mono)',
  marginLeft: 'auto',
}

const helpStyle: React.CSSProperties = {
  color: 'var(--color-muted)',
  fontSize: '0.72rem',
  margin: '0.2rem 0 0',
}

const defaultStyle: React.CSSProperties = {
  color: 'var(--color-muted)',
  fontSize: '0.68rem',
  margin: '0.15rem 0 0',
}

const subheadingStyle: React.CSSProperties = {
  fontSize: '0.68rem',
  letterSpacing: '0.12em',
  textTransform: 'uppercase',
  color: 'var(--color-neon)',
  marginBottom: '0.5rem',
  marginTop: '0.25rem',
}

const inputStyle: React.CSSProperties = {
  background: 'var(--color-bg)',
  border: '1px solid var(--color-border)',
  color: 'var(--color-text)',
  fontFamily: 'var(--font-mono)',
  fontSize: '0.74rem',
  padding: '0.35rem 0.5rem',
  borderRadius: 'var(--radius-panel)',
  outline: 'none',
}

const selectStyle: React.CSSProperties = {
  ...inputStyle,
  cursor: 'pointer',
}

const chipStyle: React.CSSProperties = {
  display: 'inline-flex',
  alignItems: 'center',
  gap: '0.4rem',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius-panel)',
  padding: '0.2rem 0.5rem',
  fontFamily: 'var(--font-mono)',
  fontSize: '0.72rem',
  color: 'var(--color-text)',
}

const chipXStyle: React.CSSProperties = {
  background: 'none',
  border: 'none',
  color: 'var(--color-alert)',
  cursor: 'pointer',
  fontSize: '0.9rem',
  lineHeight: 1,
  padding: 0,
}

function tristateChip(active: boolean): React.CSSProperties {
  return {
    padding: '0.25rem 0.6rem',
    fontSize: '0.68rem',
    fontFamily: 'var(--font-mono)',
    letterSpacing: '0.05em',
    cursor: 'pointer',
    borderRadius: 'var(--radius-panel)',
    border: `1px solid ${active ? 'var(--color-neon)' : 'var(--color-border)'}`,
    background: active ? 'color-mix(in srgb, var(--color-neon) 16%, transparent)' : 'transparent',
    color: active ? 'var(--color-neon)' : 'var(--color-muted)',
  }
}
