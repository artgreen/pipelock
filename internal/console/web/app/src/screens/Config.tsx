import { useCallback, useEffect, useMemo, useState } from 'react'
import CodeMirror from '@uiw/react-codemirror'
import { yaml as yamlLang } from '@codemirror/lang-yaml'
import { ApiError, applyConfig, getConfig, validateConfig, type ValidateResult } from '../api'
import { cyberCodeMirror } from '../lib/cmTheme'
import { hasChanges, lineDiff, readTopLevelScalar, setTopLevelScalar, type DiffLine } from '../lib/yamlpatch'
import ScreenHeader from '../components/ScreenHeader'
import Banner from '../components/Banner'
import { useToast } from '../components/toast-context'

const MODES = ['strict', 'balanced', 'audit', 'permissive'] as const

export default function Config() {
  const toast = useToast()
  const [original, setOriginal] = useState('')
  const [buffer, setBuffer] = useState('')
  const [loadError, setLoadError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [validation, setValidation] = useState<ValidateResult | null>(null)
  const [validating, setValidating] = useState(false)
  const [applying, setApplying] = useState(false)
  const [showDiff, setShowDiff] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const text = await getConfig()
      setOriginal(text)
      setBuffer(text)
      setLoadError(null)
      setValidation(null)
    } catch (e) {
      setLoadError(e instanceof Error ? e.message : 'failed to load config')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load()
  }, [load])

  const dirty = hasChanges(original, buffer)
  const mode = useMemo(() => readTopLevelScalar(buffer, 'mode'), [buffer])
  const enforce = useMemo(() => readTopLevelScalar(buffer, 'enforce'), [buffer])

  const onValidate = async () => {
    setValidating(true)
    try {
      const res = await validateConfig(buffer)
      setValidation(res)
    } catch (e) {
      setValidation({ ok: false, error: e instanceof Error ? e.message : 'validation request failed' })
    } finally {
      setValidating(false)
    }
  }

  const onApply = async () => {
    setApplying(true)
    try {
      await applyConfig(buffer)
      toast.push('applied — pipelock will hot-reload', 'ok')
      await load()
    } catch (e) {
      if (e instanceof ApiError && e.status === 400) {
        toast.push('rejected: invalid config', 'alert')
        setValidation({ ok: false, error: e.body || 'config rejected (400)' })
      } else if (e instanceof ApiError && e.status === 500) {
        toast.push('server error writing config', 'alert')
        setValidation({ ok: false, error: e.body || 'write/IO error (500)' })
      } else {
        toast.push(e instanceof Error ? e.message : 'apply failed', 'alert')
      }
    } finally {
      setApplying(false)
    }
  }

  // Quick-toggle: patch a single top-level key, then validate+apply through the
  // same path so the backend remains the source of truth.
  const applyQuickToggle = async (key: string, value: string) => {
    const patched = setTopLevelScalar(buffer, key, value)
    setBuffer(patched)
    setApplying(true)
    try {
      const v = await validateConfig(patched)
      if (!v.ok) {
        setValidation(v)
        toast.push(`${key} change failed validation`, 'alert')
        return
      }
      await applyConfig(patched)
      toast.push(`${key} → ${value} applied — pipelock will hot-reload`, 'ok')
      await load()
    } catch (e) {
      const reason = e instanceof ApiError ? e.body || e.message : e instanceof Error ? e.message : 'failed'
      setValidation({ ok: false, error: reason })
      toast.push(`${key} change rejected`, 'alert')
    } finally {
      setApplying(false)
    }
  }

  const busy = applying || validating

  return (
    <div style={{ padding: '1.5rem 1.75rem', height: '100%', display: 'flex', flexDirection: 'column', minHeight: 0 }}>
      <ScreenHeader
        title="Config"
        tag="runtime configuration"
        right={
          <>
            <button type="button" className="btn-neon" onClick={() => setShowDiff((d) => !d)} disabled={!dirty} style={{ opacity: dirty ? 1 : 0.4 }}>
              {showDiff ? 'hide diff' : 'show diff'}
            </button>
            <button type="button" className="btn-neon" onClick={onValidate} disabled={busy}>
              {validating ? '…validating' : 'validate'}
            </button>
            <button type="button" className="btn-alert" onClick={onApply} disabled={busy || !dirty} style={{ opacity: busy || !dirty ? 0.5 : 1 }}>
              {applying ? '…applying' : 'apply'}
            </button>
          </>
        }
      />

      {loadError && (
        <div style={{ marginBottom: '1rem' }}>
          <Banner tone="alert" onRetry={load}>
            Failed to load current config from pipelock: {loadError}
          </Banner>
        </div>
      )}

      {/* Quick toggles */}
      <div className="panel" style={{ marginBottom: '0.9rem', display: 'flex', gap: '1.5rem', alignItems: 'center', flexWrap: 'wrap' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem' }}>
          <span style={{ color: 'var(--color-muted)', fontSize: '0.64rem', letterSpacing: '0.12em', textTransform: 'uppercase' }}>mode</span>
          <div style={{ display: 'flex', gap: '0.25rem' }}>
            {MODES.map((m) => (
              <button
                key={m}
                type="button"
                disabled={busy}
                onClick={() => mode !== m && applyQuickToggle('mode', m)}
                style={modeChip(mode === m)}
              >
                {m}
              </button>
            ))}
          </div>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem' }}>
          <span style={{ color: 'var(--color-muted)', fontSize: '0.64rem', letterSpacing: '0.12em', textTransform: 'uppercase' }}>enforce</span>
          <button
            type="button"
            disabled={busy}
            onClick={() => applyQuickToggle('enforce', enforce === 'true' ? 'false' : 'true')}
            style={toggleStyle(enforce === 'true')}
            aria-pressed={enforce === 'true'}
          >
            <span style={{ ...knobStyle, transform: enforce === 'true' ? 'translateX(20px)' : 'translateX(0)' }} />
            <span style={{ position: 'absolute', left: enforce === 'true' ? '8px' : 'auto', right: enforce === 'true' ? 'auto' : '8px', fontSize: '0.58rem', color: enforce === 'true' ? 'var(--color-bg)' : 'var(--color-muted)', letterSpacing: '0.08em' }}>
              {enforce === 'true' ? 'ON' : enforce === undefined ? '·' : 'OFF'}
            </span>
          </button>
        </div>
        <span style={{ marginLeft: 'auto', color: 'var(--color-muted)', fontSize: '0.68rem' }}>
          quick-toggles validate + apply immediately
        </span>
      </div>

      {/* Validation result */}
      {validation && (
        <div style={{ marginBottom: '0.9rem' }}>
          {validation.ok ? (
            <Banner tone="info">config OK ✓ {validation.warnings && validation.warnings.length > 0 ? `— ${validation.warnings.length} warning(s) below` : '— no warnings'}</Banner>
          ) : (
            <Banner tone="alert">validation failed: {validation.error || 'unknown error'}</Banner>
          )}
          {validation.warnings && validation.warnings.length > 0 && (
            <div className="panel" style={{ marginTop: '0.5rem', borderColor: '#5a5000' }}>
              {validation.warnings.map((w, i) => (
                <div key={i} style={{ color: 'var(--color-warn)', fontSize: '0.74rem' }}>▲ {w}</div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Editor + optional diff */}
      <div style={{ flex: 1, minHeight: 0, display: 'grid', gridTemplateColumns: showDiff ? '1fr 1fr' : '1fr', gap: '0.9rem' }}>
        <div className="panel panel--neon" style={{ padding: '0', overflow: 'hidden', display: 'flex', flexDirection: 'column', minHeight: 0 }}>
          <div style={editorBar}>
            <span>pipelock.yaml {dirty && <span style={{ color: 'var(--color-warn)' }}>● modified</span>}</span>
            <span style={{ color: 'var(--color-muted)' }}>{loading ? 'loading…' : `${buffer.split('\n').length} lines`}</span>
          </div>
          <div style={{ flex: 1, minHeight: 0, overflow: 'hidden' }}>
            <CodeMirror
              value={buffer}
              onChange={setBuffer}
              theme="dark"
              extensions={[yamlLang(), cyberCodeMirror]}
              height="100%"
              style={{ height: '100%' }}
              basicSetup={{ lineNumbers: true, foldGutter: true, highlightActiveLine: true }}
            />
          </div>
        </div>

        {showDiff && (
          <div className="panel" style={{ padding: 0, overflow: 'hidden', display: 'flex', flexDirection: 'column', minHeight: 0 }}>
            <div style={editorBar}>
              <span>diff — current vs edited</span>
            </div>
            <DiffView diff={lineDiff(original, buffer)} />
          </div>
        )}
      </div>
    </div>
  )
}

function DiffView({ diff }: { diff: DiffLine[] }) {
  return (
    <div style={{ flex: 1, overflow: 'auto', minHeight: 0, fontSize: '0.74rem', lineHeight: 1.6 }}>
      {diff.map((d, i) => {
        const bg = d.kind === 'add' ? 'rgba(57,255,20,0.10)' : d.kind === 'del' ? 'rgba(255,45,45,0.12)' : 'transparent'
        const sign = d.kind === 'add' ? '+' : d.kind === 'del' ? '-' : ' '
        const color = d.kind === 'add' ? 'var(--color-neon)' : d.kind === 'del' ? 'var(--color-alert)' : 'var(--color-muted)'
        return (
          <div key={i} style={{ display: 'flex', background: bg, padding: '0 0.5rem', whiteSpace: 'pre' }}>
            <span style={{ width: '1.2rem', color, flexShrink: 0, userSelect: 'none' }}>{sign}</span>
            <span style={{ color: d.kind === 'same' ? 'var(--color-text)' : color, overflow: 'hidden' }}>{d.text || ' '}</span>
          </div>
        )
      })}
    </div>
  )
}

const editorBar: React.CSSProperties = {
  display: 'flex',
  justifyContent: 'space-between',
  padding: '0.4rem 0.75rem',
  borderBottom: '1px solid var(--color-border)',
  fontSize: '0.66rem',
  letterSpacing: '0.08em',
  textTransform: 'uppercase',
  color: 'var(--color-text)',
  flexShrink: 0,
  background: 'var(--color-surface)',
}

function modeChip(active: boolean): React.CSSProperties {
  return {
    padding: '0.3rem 0.65rem',
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

function toggleStyle(on: boolean): React.CSSProperties {
  return {
    position: 'relative',
    width: '46px',
    height: '24px',
    borderRadius: '12px',
    border: `1px solid ${on ? 'var(--color-neon)' : 'var(--color-border)'}`,
    background: on ? 'color-mix(in srgb, var(--color-neon) 30%, transparent)' : 'var(--color-bg)',
    cursor: 'pointer',
    padding: 0,
    display: 'flex',
    alignItems: 'center',
  }
}

const knobStyle: React.CSSProperties = {
  position: 'absolute',
  left: '2px',
  width: '18px',
  height: '18px',
  borderRadius: '50%',
  background: 'var(--color-neon)',
  boxShadow: '0 0 6px var(--color-neon)',
  transition: 'transform 0.18s',
}
