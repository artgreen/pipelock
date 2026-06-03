import { useCallback, useEffect, useRef, useState } from 'react'
import {
  ApiError,
  applyConfigStructured,
  getConfigSchema,
  getConfigValues,
  type ConfigSchema,
  type ConfigValues,
  type SchemaField,
} from '../../api'
import Banner from '../../components/Banner'
import { useToast } from '../../components/toast-context'
import Field from './Field'
import SectionTree from './SectionTree'
import { getPath } from './fieldvalue'

// ─── Props ────────────────────────────────────────────────────────────────────

interface Props {
  onOpenAdvanced?: () => void
}

// ─── Component ────────────────────────────────────────────────────────────────

export default function AllSettings({ onOpenAdvanced }: Props) {
  const toast = useToast()

  // ── Fetch state ──
  const [schema, setSchema] = useState<ConfigSchema | null>(null)
  const [values, setValues] = useState<ConfigValues | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<string | null>(null)

  // ── Navigation state ──
  const [selectedSection, setSelectedSection] = useState<string>('')
  const [query, setQuery] = useState('')

  // ── Edit state ──
  const [changes, setChanges] = useState<Record<string, unknown>>({})

  // ── Save state ──
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)

  // ── Highlight state (scroll target from search) ──
  const [highlightPath, setHighlightPath] = useState<string | null>(null)
  const highlightRef = useRef<HTMLDivElement | null>(null)

  // ── Load ──

  const loadValues = useCallback(async () => {
    try {
      const v = await getConfigValues()
      setValues(v)
    } catch (e) {
      // loadError already set by the parallel load; just note the failure here.
      setLoadError(e instanceof Error ? e.message : 'failed to load config values')
    }
  }, [])

  const load = useCallback(async () => {
    setLoading(true)
    setLoadError(null)
    try {
      const [s, v] = await Promise.all([getConfigSchema(), getConfigValues()])
      setSchema(s)
      setValues(v)
      setSelectedSection((prev) => {
        if (prev) return prev
        return s.sections.length > 0 ? s.sections[0].key : ''
      })
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

  // ── Highlight scroll effect ──

  useEffect(() => {
    if (highlightPath && highlightRef.current) {
      highlightRef.current.scrollIntoView({ behavior: 'smooth', block: 'center' })
    }
  }, [highlightPath])

  // ── Handlers ──

  const handleSelectField = useCallback((sectionKey: string, path: string) => {
    setSelectedSection(sectionKey)
    setQuery('')
    setHighlightPath(path)
  }, [])

  const handleChange = useCallback((path: string, value: unknown) => {
    setChanges((prev) => ({ ...prev, [path]: value }))
    setSaveError(null)
  }, [])

  const handleDiscard = useCallback(() => {
    setChanges({})
    setSaveError(null)
  }, [])

  const handleSave = useCallback(async () => {
    if (Object.keys(changes).length === 0) return
    setSaving(true)
    setSaveError(null)
    try {
      await applyConfigStructured(changes)
      toast.push('saved — pipelock will hot-reload', 'ok')
      setChanges({})
      await loadValues()
    } catch (e) {
      if (e instanceof ApiError && e.status === 400) {
        setSaveError(e.body || 'config rejected (400)')
      } else {
        toast.push(e instanceof Error ? e.message : 'save failed', 'alert')
      }
    } finally {
      setSaving(false)
    }
  }, [changes, loadValues, toast])

  // ── Derived ──

  const pendingCount = Object.keys(changes).length
  const selectedTopLevel =
    schema?.sections.find((s) => s.key === selectedSection) ?? null

  // ── Render: loading / error ──

  if (loading) {
    return (
      <div style={outerStyle}>
        <span style={{ color: 'var(--color-muted)', fontSize: '0.78rem' }}>loading settings…</span>
      </div>
    )
  }

  if (loadError || !schema || !values) {
    return (
      <div style={outerStyle}>
        <Banner tone="alert" onRetry={load}>
          Failed to load settings: {loadError ?? 'unknown error'}
        </Banner>
      </div>
    )
  }

  // ── Render: main ──

  return (
    <div style={outerStyle}>
      {/* ── Two-column layout ── */}
      <div style={layoutStyle}>
        {/* Left: section tree */}
        <div style={sidebarStyle}>
          <SectionTree
            sections={schema.sections}
            selected={selectedSection}
            query={query}
            onSelect={(key) => {
              setSelectedSection(key)
              setHighlightPath(null)
            }}
            onQuery={setQuery}
            onSelectField={handleSelectField}
          />
        </div>

        {/* Right: fields */}
        <div style={contentStyle}>
          {selectedTopLevel ? (
            <SectionPane
              section={selectedTopLevel}
              values={values}
              changes={changes}
              highlightPath={highlightPath}
              highlightRef={highlightRef}
              onChange={handleChange}
              onAdvanced={onOpenAdvanced}
            />
          ) : (
            <span style={{ color: 'var(--color-muted)', fontSize: '0.78rem' }}>select a section</span>
          )}
        </div>
      </div>

      {/* ── Sticky save bar ── */}
      <div style={saveBarStyle}>
        {saveError && (
          <div style={saveErrorStyle}>
            <Banner tone="alert">config rejected: {saveError}</Banner>
          </div>
        )}
        <div style={saveBarInnerStyle}>
          <span style={pendingLabelStyle}>
            {pendingCount === 0
              ? 'no pending changes'
              : `${pendingCount} pending change${pendingCount === 1 ? '' : 's'}`}
          </span>
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <button
              type="button"
              className="btn-neon"
              disabled={pendingCount === 0 || saving}
              onClick={handleDiscard}
              style={{ opacity: pendingCount === 0 || saving ? 0.4 : 1 }}
            >
              discard
            </button>
            <button
              type="button"
              className="btn-alert"
              disabled={pendingCount === 0 || saving}
              onClick={() => void handleSave()}
              style={{ opacity: pendingCount === 0 || saving ? 0.5 : 1 }}
            >
              {saving ? '…saving' : 'save'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

// ─── SectionPane ──────────────────────────────────────────────────────────────

interface SectionPaneProps {
  section: SchemaField
  values: ConfigValues
  changes: Record<string, unknown>
  highlightPath: string | null
  highlightRef: React.RefObject<HTMLDivElement | null>
  onChange: (path: string, value: unknown) => void
  onAdvanced?: () => void
}

function SectionPane({
  section,
  values,
  changes,
  highlightPath,
  highlightRef,
  onChange,
  onAdvanced,
}: SectionPaneProps) {
  const fields = section.children ?? []

  if (fields.length === 0) {
    return (
      <span style={{ color: 'var(--color-muted)', fontSize: '0.78rem' }}>
        no configurable fields in this section
      </span>
    )
  }

  return (
    <div>
      <div style={sectionHeadingStyle}>{section.label}</div>
      {fields.map((field) => {
        const effectiveValue =
          field.path in changes
            ? changes[field.path]
            : getPath(values.effective, field.path)
        const present = values.present[field.path] === true
        const isHighlighted = highlightPath === field.path

        return (
          <div
            key={field.path}
            ref={isHighlighted ? highlightRef : undefined}
            style={isHighlighted ? highlightWrapStyle : undefined}
          >
            <Field
              field={field}
              value={effectiveValue}
              present={present}
              onChange={onChange}
              onAdvanced={onAdvanced}
            />
          </div>
        )
      })}
    </div>
  )
}

// ─── Styles ───────────────────────────────────────────────────────────────────

const outerStyle: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  height: '100%',
  minHeight: 0,
}

const layoutStyle: React.CSSProperties = {
  display: 'flex',
  flex: 1,
  minHeight: 0,
  gap: '0',
}

const sidebarStyle: React.CSSProperties = {
  width: '240px',
  flexShrink: 0,
  borderRight: '1px solid var(--color-border)',
  overflowY: 'auto',
  paddingRight: '0.5rem',
  paddingTop: '0.25rem',
}

const contentStyle: React.CSSProperties = {
  flex: 1,
  minWidth: 0,
  overflowY: 'auto',
  padding: '0.5rem 1.25rem 1.5rem',
}

const sectionHeadingStyle: React.CSSProperties = {
  fontSize: '0.64rem',
  letterSpacing: '0.14em',
  textTransform: 'uppercase',
  color: 'var(--color-neon)',
  marginBottom: '1rem',
  marginTop: '0.25rem',
  paddingBottom: '0.4rem',
  borderBottom: '1px solid var(--color-border)',
}

const highlightWrapStyle: React.CSSProperties = {
  background: 'color-mix(in srgb, var(--color-neon) 6%, transparent)',
  borderRadius: 'var(--radius-panel)',
  padding: '0.25rem 0.5rem',
  marginLeft: '-0.5rem',
  marginRight: '-0.5rem',
}

const saveBarStyle: React.CSSProperties = {
  flexShrink: 0,
  borderTop: '1px solid var(--color-border)',
  background: 'var(--color-surface)',
}

const saveBarInnerStyle: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  padding: '0.6rem 1rem',
}

const saveErrorStyle: React.CSSProperties = {
  padding: '0.5rem 1rem 0',
}

const pendingLabelStyle: React.CSSProperties = {
  color: 'var(--color-muted)',
  fontSize: '0.7rem',
  fontFamily: 'var(--font-mono)',
  letterSpacing: '0.04em',
}
