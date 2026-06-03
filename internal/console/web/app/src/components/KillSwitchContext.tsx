import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react'
import { getKillSwitch, setKillSwitch, type KillSwitch } from '../api'

interface KillSwitchState {
  data: KillSwitch | null
  loading: boolean
  error: string | null
  // refresh respects the backend rate limit (10/min): hard floor of 15s between calls.
  refresh: () => Promise<void>
  toggle: (active: boolean) => Promise<void>
}

const Ctx = createContext<KillSwitchState | null>(null)

export function useKillSwitch(): KillSwitchState {
  const ctx = useContext(Ctx)
  if (!ctx) throw new Error('useKillSwitch must be used within KillSwitchProvider')
  return ctx
}

const MIN_INTERVAL_MS = 15_000

export function KillSwitchProvider({ children }: { children: ReactNode }) {
  const [data, setData] = useState<KillSwitch | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const lastFetch = useRef(0)

  const refresh = useCallback(async () => {
    const now = Date.now()
    if (now - lastFetch.current < MIN_INTERVAL_MS) return // rate-limit guard
    lastFetch.current = now
    setLoading(true)
    try {
      const ks = await getKillSwitch()
      setData(ks)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'failed to load kill switch')
    } finally {
      setLoading(false)
    }
  }, [])

  const toggle = useCallback(async (active: boolean) => {
    await setKillSwitch(active)
    // Force a fresh read after a toggle, bypassing the interval guard.
    lastFetch.current = Date.now()
    try {
      const ks = await getKillSwitch()
      setData(ks)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'failed to refresh kill switch')
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  return <Ctx.Provider value={{ data, loading, error, refresh, toggle }}>{children}</Ctx.Provider>
}
