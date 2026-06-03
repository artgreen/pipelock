import { type SchemaField } from '../../api'

interface SearchHit {
  path: string        // dotted path of the matching leaf field
  label: string
  help: string        // field help text (may be empty)
  sectionKey: string  // the top-level section key it belongs to
  sectionLabel: string
}

interface Props {
  sections: SchemaField[]     // top-level sections (from schema.sections)
  selected: string            // selected top-level section key
  query: string
  onSelect: (sectionKey: string) => void
  onQuery: (q: string) => void
  onSelectField?: (sectionKey: string, path: string) => void // jump to a search hit
}

// ─── Recursive leaf walker ────────────────────────────────────────────────────

function collectLeaves(field: SchemaField, sectionKey: string, sectionLabel: string, acc: SearchHit[]): void {
  if (field.type !== 'group') {
    acc.push({ path: field.path, label: field.label, help: field.help ?? '', sectionKey, sectionLabel })
    return
  }
  if (field.children) {
    for (const child of field.children) {
      collectLeaves(child, sectionKey, sectionLabel, acc)
    }
  }
}

function buildLeafIndex(sections: SchemaField[]): SearchHit[] {
  const out: SearchHit[] = []
  for (const section of sections) {
    if (section.children) {
      for (const child of section.children) {
        collectLeaves(child, section.key, section.label, out)
      }
    }
  }
  return out
}

const MAX_HITS = 50

function searchLeaves(leaves: SearchHit[], query: string): SearchHit[] {
  const q = query.toLowerCase()
  const hits: SearchHit[] = []
  for (const leaf of leaves) {
    if (
      leaf.path.toLowerCase().includes(q) ||
      leaf.label.toLowerCase().includes(q) ||
      leaf.help.toLowerCase().includes(q)
    ) {
      hits.push(leaf)
      if (hits.length >= MAX_HITS) break
    }
  }
  return hits
}

// ─── Component ────────────────────────────────────────────────────────────────

export default function SectionTree({
  sections,
  selected,
  query,
  onSelect,
  onQuery,
  onSelectField,
}: Props) {
  const isSearching = query.length >= 2

  // Build leaf index lazily — only when search is active. This is cheap enough
  // for the config schema (O(field_count)) that we don't need useMemo here.
  const hits = isSearching ? searchLeaves(buildLeafIndex(sections), query) : []

  return (
    <div style={containerStyle}>
      {/* ── Search input ── */}
      <div style={searchWrapStyle}>
        <input
          type="search"
          value={query}
          onChange={(e) => onQuery(e.target.value)}
          placeholder="search settings…"
          aria-label="Search settings"
          style={searchInputStyle}
        />
      </div>

      {/* ── Nav: section list or search results ── */}
      {!isSearching ? (
        <nav aria-label="Config sections">
          {sections.map((section) => {
            const active = section.key === selected
            return (
              <button
                key={section.key}
                type="button"
                onClick={() => onSelect(section.key)}
                style={navItemStyle(active)}
                aria-current={active ? 'page' : undefined}
              >
                {section.label}
              </button>
            )
          })}
        </nav>
      ) : (
        <div role="list" aria-label="Search results">
          {hits.length === 0 ? (
            <p style={noMatchStyle}>no matches</p>
          ) : (
            hits.map((hit) => (
              <button
                key={hit.path}
                type="button"
                role="listitem"
                onClick={() => {
                  if (onSelectField) {
                    onSelectField(hit.sectionKey, hit.path)
                  } else {
                    onSelect(hit.sectionKey)
                  }
                }}
                style={hitItemStyle}
              >
                <span style={hitLabelStyle}>
                  <span style={hitSectionStyle}>{hit.sectionLabel}</span>
                  <span style={hitSepStyle}> › </span>
                  {hit.label}
                </span>
                <code style={hitPathStyle}>{hit.path}</code>
              </button>
            ))
          )}
        </div>
      )}
    </div>
  )
}

// ─── Styles ───────────────────────────────────────────────────────────────────

const containerStyle: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: 0,
  width: '100%',
}

const searchWrapStyle: React.CSSProperties = {
  padding: '0.5rem 0',
  marginBottom: '0.25rem',
}

const searchInputStyle: React.CSSProperties = {
  width: '100%',
  background: 'var(--color-bg)',
  border: '1px solid var(--color-border)',
  color: 'var(--color-text)',
  fontFamily: 'var(--font-mono)',
  fontSize: '0.74rem',
  padding: '0.35rem 0.5rem',
  borderRadius: 'var(--radius-panel)',
  outline: 'none',
}

function navItemStyle(active: boolean): React.CSSProperties {
  return {
    display: 'block',
    width: '100%',
    textAlign: 'left',
    background: active
      ? 'color-mix(in srgb, var(--color-neon) 12%, transparent)'
      : 'transparent',
    border: 'none',
    borderLeft: active
      ? '2px solid var(--color-neon)'
      : '2px solid transparent',
    color: active ? 'var(--color-neon)' : 'var(--color-text)',
    fontFamily: 'var(--font-mono)',
    fontSize: '0.74rem',
    letterSpacing: '0.04em',
    padding: '0.4rem 0.6rem',
    cursor: 'pointer',
    borderRadius: '0 var(--radius-panel) var(--radius-panel) 0',
    transition: 'background 0.1s, color 0.1s',
  }
}

const hitItemStyle: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: '0.15rem',
  width: '100%',
  textAlign: 'left',
  background: 'transparent',
  border: 'none',
  borderBottom: '1px solid var(--color-border)',
  color: 'var(--color-text)',
  fontFamily: 'var(--font-mono)',
  fontSize: '0.72rem',
  padding: '0.45rem 0.5rem',
  cursor: 'pointer',
}

const hitLabelStyle: React.CSSProperties = {
  fontSize: '0.72rem',
  color: 'var(--color-text)',
}

const hitSectionStyle: React.CSSProperties = {
  color: 'var(--color-muted)',
  fontSize: '0.68rem',
}

const hitSepStyle: React.CSSProperties = {
  color: 'var(--color-muted)',
}

const hitPathStyle: React.CSSProperties = {
  color: 'var(--color-muted)',
  fontSize: '0.62rem',
  fontFamily: 'var(--font-mono)',
}

const noMatchStyle: React.CSSProperties = {
  color: 'var(--color-muted)',
  fontSize: '0.72rem',
  padding: '0.5rem 0.5rem',
  margin: 0,
}
