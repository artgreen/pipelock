// Minimal, dependency-free helpers for the Config screen.
// We deliberately do NOT parse YAML in the browser — the backend is the source
// of truth and validates everything. These helpers only do surgical, top-level
// scalar edits and a naive line diff for preview. The validate+apply round trip
// is what actually guarantees correctness.

// Read a top-level scalar key's value (best-effort; returns undefined if absent
// or if the key appears nested). Only matches keys at column 0.
export function readTopLevelScalar(yaml: string, key: string): string | undefined {
  const re = new RegExp(`^${escapeKey(key)}:[ \\t]*(.*)$`, 'm')
  const m = yaml.match(re)
  if (!m) return undefined
  let v = m[1].trim()
  // Strip surrounding quotes and trailing comments.
  v = v.replace(/\s+#.*$/, '').trim()
  if ((v.startsWith('"') && v.endsWith('"')) || (v.startsWith("'") && v.endsWith("'"))) {
    v = v.slice(1, -1)
  }
  return v
}

// Set a top-level scalar key. Replaces the existing top-level line if present
// (preserving any inline comment), otherwise prepends the key. Values that need
// no quoting (simple identifiers / booleans) are written bare.
export function setTopLevelScalar(yaml: string, key: string, value: string): string {
  const re = new RegExp(`^(${escapeKey(key)}:)([ \\t]*)([^\\n]*)$`, 'm')
  const rendered = renderValue(value)
  if (re.test(yaml)) {
    return yaml.replace(re, (_full, k: string, _ws: string, rest: string) => {
      const comment = rest.match(/\s#.*$/)
      return `${k} ${rendered}${comment ? comment[0] : ''}`
    })
  }
  // Not present: prepend, keeping a trailing newline.
  const prefix = `${key}: ${rendered}\n`
  return yaml.startsWith('\n') ? prefix + yaml : prefix + (yaml.length ? yaml : '')
}

function renderValue(value: string): string {
  if (/^[A-Za-z0-9_./-]+$/.test(value)) return value
  return JSON.stringify(value) // safe double-quoted form
}

function escapeKey(key: string): string {
  return key.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

// ─── Naive line diff for the preview panel ───────────────────────────────────

export type DiffLine = { kind: 'same' | 'add' | 'del'; text: string; aNum?: number; bNum?: number }

// LCS-based line diff. Inputs are small config files, so O(n*m) is fine.
export function lineDiff(a: string, b: string): DiffLine[] {
  const A = a.split('\n')
  const B = b.split('\n')
  const n = A.length
  const m = B.length
  const lcs: number[][] = Array.from({ length: n + 1 }, () => new Array(m + 1).fill(0))
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      lcs[i][j] = A[i] === B[j] ? lcs[i + 1][j + 1] + 1 : Math.max(lcs[i + 1][j], lcs[i][j + 1])
    }
  }
  const out: DiffLine[] = []
  let i = 0
  let j = 0
  let aNum = 1
  let bNum = 1
  while (i < n && j < m) {
    if (A[i] === B[j]) {
      out.push({ kind: 'same', text: A[i], aNum: aNum++, bNum: bNum++ })
      i++
      j++
    } else if (lcs[i + 1][j] >= lcs[i][j + 1]) {
      out.push({ kind: 'del', text: A[i], aNum: aNum++ })
      i++
    } else {
      out.push({ kind: 'add', text: B[j], bNum: bNum++ })
      j++
    }
  }
  while (i < n) out.push({ kind: 'del', text: A[i++], aNum: aNum++ })
  while (j < m) out.push({ kind: 'add', text: B[j++], bNum: bNum++ })
  return out
}

export function hasChanges(a: string, b: string): boolean {
  return a !== b
}

// ─── Sequence (list) helpers ─────────────────────────────────────────────────
// Surgical, line-based edits of block sequences under a top-level or
// single-nested key (e.g. "api_allowlist" or "fetch_proxy.monitoring.blocklist").
// Consistent with the scalar helpers: no full YAML parse; the backend validates.

type SeqBlock = { keyLine: number; keyIndent: number; firstItem: number; endItem: number }

function leadingSpaces(line: string): number {
  const m = line.match(/^(\s*)/)
  return m ? m[1].length : 0
}

function stripComment(s: string): string {
  return s.replace(/\s+#.*$/, '')
}

function unquote(s: string): string {
  const v = s.trim()
  if ((v.startsWith('"') && v.endsWith('"')) || (v.startsWith("'") && v.endsWith("'"))) return v.slice(1, -1)
  return v
}

// Locate the key line for a dotted path by walking segments with increasing
// indent. Returns undefined if any segment is missing.
function findKeyLine(lines: string[], path: string): number | undefined {
  const segs = path.split('.')
  let start = 0
  let parentIndent = -1
  let found = -1
  for (let s = 0; s < segs.length; s++) {
    const re = new RegExp(`^(\\s*)${escapeKey(segs[s])}:`)
    found = -1
    for (let i = start; i < lines.length; i++) {
      const m = lines[i].match(re)
      if (!m) continue
      const indent = m[1].length
      if (indent <= parentIndent) continue
      found = i
      parentIndent = indent
      start = i + 1
      break
    }
    if (found === -1) return undefined
  }
  return found
}

function locateSeq(yaml: string, path: string): SeqBlock | undefined {
  const lines = yaml.split('\n')
  const keyLine = findKeyLine(lines, path)
  if (keyLine === undefined) return undefined
  const keyIndent = leadingSpaces(lines[keyLine])
  let firstItem = -1
  let endItem = keyLine
  for (let i = keyLine + 1; i < lines.length; i++) {
    const line = lines[i]
    if (line.trim() === '') {
      endItem = i
      continue
    }
    const indent = leadingSpaces(line)
    if (indent <= keyIndent) break
    if (/^\s*-\s+/.test(line)) {
      if (firstItem === -1) firstItem = i
      endItem = i
    } else if (line.trim().startsWith('#')) {
      endItem = i
    } else {
      break
    }
  }
  return { keyLine, keyIndent, firstItem, endItem }
}

export function readSequence(yaml: string, path: string): string[] {
  const loc = locateSeq(yaml, path)
  if (!loc || loc.firstItem === -1) return []
  const lines = yaml.split('\n')
  const items: string[] = []
  for (let i = loc.firstItem; i <= loc.endItem; i++) {
    const m = lines[i].match(/^\s*-\s+(.*)$/)
    if (m) items.push(unquote(stripComment(m[1])))
  }
  return items
}

export function addToSequence(yaml: string, path: string, value: string): string {
  if (readSequence(yaml, path).includes(value)) return yaml // idempotent
  const lines = yaml.split('\n')
  const loc = locateSeq(yaml, path)
  const rendered = renderValue(value)
  if (loc && loc.firstItem !== -1) {
    const itemIndent = ' '.repeat(loc.keyIndent + 2)
    lines.splice(loc.endItem + 1, 0, `${itemIndent}- ${rendered}`)
    return lines.join('\n')
  }
  if (loc) {
    // Key exists but has no items yet (e.g. "api_allowlist: []" or empty).
    // Replace an inline "[]" if present, else insert a first item line.
    const itemIndent = ' '.repeat(loc.keyIndent + 2)
    const keyText = lines[loc.keyLine]
    if (/:\s*\[\s*\]/.test(keyText)) {
      lines[loc.keyLine] = keyText.replace(/:\s*\[\s*\]/, ':')
    }
    lines.splice(loc.keyLine + 1, 0, `${itemIndent}- ${rendered}`)
    return lines.join('\n')
  }
  // Section/key absent: append a new top-level block (single-nested paths only).
  const segs = path.split('.')
  const trailingNL = yaml.endsWith('\n') ? '' : '\n'
  let block = trailingNL
  for (let i = 0; i < segs.length - 1; i++) block += `${'  '.repeat(i)}${segs[i]}:\n`
  const leafIndent = '  '.repeat(segs.length - 1)
  block += `${leafIndent}${segs[segs.length - 1]}:\n${leafIndent}  - ${rendered}\n`
  return yaml + block
}

export function removeFromSequence(yaml: string, path: string, value: string): string {
  const loc = locateSeq(yaml, path)
  if (!loc || loc.firstItem === -1) return yaml
  const lines = yaml.split('\n')
  for (let i = loc.firstItem; i <= loc.endItem; i++) {
    const m = lines[i].match(/^\s*-\s+(.*)$/)
    if (m && unquote(stripComment(m[1])) === value) {
      lines.splice(i, 1)
      return lines.join('\n')
    }
  }
  return yaml
}
