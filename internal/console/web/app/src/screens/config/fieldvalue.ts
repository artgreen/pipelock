// Pure helpers for the settings field renderer. No React.

// getPath reads a dotted path out of a nested object (the /values "effective"
// map), returning undefined if any segment is missing.
export function getPath(obj: Record<string, unknown>, path: string): unknown {
  let cur: unknown = obj
  for (const seg of path.split('.')) {
    if (cur == null || typeof cur !== 'object') return undefined
    cur = (cur as Record<string, unknown>)[seg]
  }
  return cur
}

// coerce converts a raw input value to the type the backend expects.
export function coerce(type: string, raw: unknown): unknown {
  switch (type) {
    case 'int': return typeof raw === 'string' ? parseInt(raw, 10) : raw
    case 'float': return typeof raw === 'string' ? parseFloat(raw as string) : raw
    default: return raw
  }
}
