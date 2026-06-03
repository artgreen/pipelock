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
