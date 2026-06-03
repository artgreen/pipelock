// Typed API client for the pipelock-console backend.
// All requests are same-origin JSON and send the session cookie.
// Any 401 on a protected call triggers a redirect to /login.

import { useEffect, useRef, useState } from 'react'

// ─── Wire types ──────────────────────────────────────────────────────────────

export interface SetupStatus {
  needs_setup: boolean
}

export interface NamedCount {
  name: string
  count: number
}

export interface Stats {
  uptime_seconds: number
  requests: {
    total: number
    allowed: number
    blocked: number
    block_rate: number
  }
  tunnels: number
  websockets: number
  top_blocked_domains: NamedCount[] | null
  top_scanners: NamedCount[] | null
  sessions: {
    active: number
    anomalies: number
    escalations: number
  }
}

export interface SessionsResponse {
  sessions: Record<string, unknown>[]
  count: number
}

export interface KillSwitch {
  active: boolean
  sources: Record<string, boolean>
  message?: string
}

export interface ValidateResult {
  ok: boolean
  error?: string
  warnings?: string[]
}

export interface ServiceStatus {
  status: string
}

export interface RestartResult {
  output: string
}

export interface PipelockEvent {
  severity: string
  type: string
  timestamp: string
  pipelock_instance: string
  fields: Record<string, unknown>
}

export interface UnblockProposal {
  op: 'list_add' | 'list_remove'
  path: string
  value: string
  explanation: string
  still_scanned: string[]
  warning?: string
}

export interface SchemaField {
  path: string
  key: string
  label: string
  type: 'group' | 'bool' | 'tristate' | 'int' | 'float' | 'string' | 'enum' | 'list' | 'map' | 'objlist' | 'objmap' | 'opaque'
  help?: string
  default?: unknown
  enum?: string[]
  secret?: boolean
  advanced_only?: boolean
  children?: SchemaField[]
  element?: SchemaField[]
}
export interface ConfigSchema {
  field_count: number
  sections: SchemaField[]
}
export interface ConfigValues {
  effective: Record<string, unknown>
  present: Record<string, boolean>
}
export const REDACTED_SENTINEL = '__redacted__'

// ─── Auth-redirect plumbing ──────────────────────────────────────────────────

// The router installs this so api calls can navigate on 401 without importing
// the router. Falls back to a hard redirect if not installed yet.
let unauthorizedHandler: (() => void) | null = null

export function setUnauthorizedHandler(fn: (() => void) | null): void {
  unauthorizedHandler = fn
}

export class ApiError extends Error {
  status: number
  body: string
  constructor(status: number, body: string) {
    super(`api ${status}: ${body || '(no body)'}`)
    this.status = status
    this.body = body
  }
}

function handleUnauthorized(): void {
  if (unauthorizedHandler) {
    unauthorizedHandler()
  } else if (typeof window !== 'undefined' && window.location.pathname !== '/login') {
    window.location.assign('/login')
  }
}

// Core fetch wrapper. Always includes credentials. On 401 → redirect + throw.
async function request(path: string, init?: RequestInit): Promise<Response> {
  let res: Response
  try {
    res = await fetch(path, { credentials: 'include', ...init })
  } catch (err) {
    throw new ApiError(0, err instanceof Error ? err.message : 'network error')
  }
  if (res.status === 401) {
    handleUnauthorized()
    throw new ApiError(401, 'unauthorized')
  }
  return res
}

async function asJSON<T>(res: Response): Promise<T> {
  const text = await res.text()
  if (!res.ok) throw new ApiError(res.status, text)
  return (text ? JSON.parse(text) : {}) as T
}

// ─── Endpoint wrappers ───────────────────────────────────────────────────────

export async function getSetup(): Promise<SetupStatus> {
  // Public endpoint — no auth redirect on its own status codes besides 401.
  return asJSON<SetupStatus>(await request('/api/setup'))
}

export async function postSetup(password: string): Promise<void> {
  const res = await request('/api/setup', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ password }),
  })
  if (!res.ok) throw new ApiError(res.status, await res.text())
}

export async function login(password: string): Promise<void> {
  const res = await request('/api/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ password }),
  })
  if (!res.ok) throw new ApiError(res.status, await res.text())
}

export async function logout(): Promise<void> {
  await request('/api/logout', { method: 'POST' })
}

export async function getStats(): Promise<Stats> {
  return asJSON<Stats>(await request('/api/stats'))
}

export async function getSessions(): Promise<SessionsResponse> {
  return asJSON<SessionsResponse>(await request('/api/sessions'))
}

export async function getKillSwitch(): Promise<KillSwitch> {
  return asJSON<KillSwitch>(await request('/api/killswitch'))
}

export async function setKillSwitch(active: boolean): Promise<void> {
  const res = await request('/api/killswitch', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ active }),
  })
  if (!res.ok) throw new ApiError(res.status, await res.text())
}

export async function getConfig(): Promise<string> {
  const res = await request('/api/config')
  const text = await res.text()
  if (!res.ok) throw new ApiError(res.status, text)
  return text
}

export async function validateConfig(yaml: string): Promise<ValidateResult> {
  const res = await request('/api/config/validate', {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-yaml' },
    body: yaml,
  })
  return asJSON<ValidateResult>(res)
}

// applyConfig: 204 on success. On 400/500 the body is the reason — surface it.
export async function applyConfig(yaml: string): Promise<void> {
  const res = await request('/api/config', {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-yaml' },
    body: yaml,
  })
  if (!res.ok) throw new ApiError(res.status, await res.text())
}

export async function getConfigSchema(): Promise<ConfigSchema> {
  return asJSON<ConfigSchema>(await request('/api/config/schema'))
}

export async function getConfigValues(): Promise<ConfigValues> {
  return asJSON<ConfigValues>(await request('/api/config/values'))
}

// applyConfigStructured posts a sparse {path: value} patch. A null value deletes
// the field (revert to default); REDACTED_SENTINEL leaves a secret unchanged.
// 400 (invalid config) throws ApiError with the reason in the body.
export async function applyConfigStructured(changes: Record<string, unknown>): Promise<void> {
  const res = await request('/api/config/structured', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ changes }),
  })
  if (!res.ok) throw new ApiError(res.status, await res.text())
}

// proposeUnblock asks the backend for the minimal config change that allows a
// blocked destination. matchedPattern is the blocklist pattern that matched (if
// known, e.g. from the event) so blocklist removals target the right entry.
// Returns the proposal; 422 (unsupported reason) throws ApiError with the reason
// in the body.
export async function proposeUnblock(target: string, reason: string, matchedPattern = ''): Promise<UnblockProposal> {
  const res = await request('/api/config/unblock-proposal', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ target, reason, matched_pattern: matchedPattern }),
  })
  return asJSON<UnblockProposal>(res)
}

export async function getService(): Promise<ServiceStatus> {
  return asJSON<ServiceStatus>(await request('/api/service'))
}

export async function restartService(): Promise<RestartResult> {
  const res = await request('/api/service/restart', { method: 'POST' })
  return asJSON<RestartResult>(res)
}

// ─── Event stream hook ───────────────────────────────────────────────────────

const EVENT_CAP = 500

function eventKey(e: PipelockEvent): string {
  // Backend may replay a buffer snapshot on connect; dedupe on a stable tuple.
  return `${e.timestamp}|${e.type}|${e.severity}|${e.pipelock_instance}`
}

export interface EventStream {
  events: PipelockEvent[]
  connected: boolean
}

// useEventStream wraps EventSource('/api/events'), returning a rolling,
// newest-first list capped at 500 with duplicates removed.
export function useEventStream(): EventStream {
  const [events, setEvents] = useState<PipelockEvent[]>([])
  const [connected, setConnected] = useState(false)
  const seen = useRef<Set<string>>(new Set())

  useEffect(() => {
    const es = new EventSource('/api/events', { withCredentials: true })

    es.onopen = () => setConnected(true)

    es.onmessage = (msg) => {
      let parsed: PipelockEvent
      try {
        parsed = JSON.parse(msg.data) as PipelockEvent
      } catch {
        return
      }
      const key = eventKey(parsed)
      if (seen.current.has(key)) return
      seen.current.add(key)

      setEvents((prev) => {
        const next = [parsed, ...prev]
        if (next.length > EVENT_CAP) {
          const dropped = next.splice(EVENT_CAP)
          for (const d of dropped) seen.current.delete(eventKey(d))
        }
        return next
      })
    }

    es.onerror = () => {
      setConnected(false)
      // EventSource auto-reconnects; if the session expired the next protected
      // fetch (stats poll) will catch the 401 and redirect.
    }

    return () => es.close()
  }, [])

  return { events, connected }
}

export const EVENT_STREAM_CAP = EVENT_CAP
