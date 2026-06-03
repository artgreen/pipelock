import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { getKillSwitch, setKillSwitch, type KillSwitch } from '../api'
import { KillSwitchContext } from './killswitch-context'

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
    // Initial load; setState happens asynchronously inside refresh().
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void refresh()
  }, [refresh])

  return <KillSwitchContext.Provider value={{ data, loading, error, refresh, toggle }}>{children}</KillSwitchContext.Provider>
}
