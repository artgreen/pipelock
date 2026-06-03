import { createContext, useContext } from 'react'
import type { KillSwitch } from '../api'

export interface KillSwitchState {
  data: KillSwitch | null
  loading: boolean
  error: string | null
  // refresh respects the backend rate limit (10/min): hard floor of 15s between calls.
  refresh: () => Promise<void>
  toggle: (active: boolean) => Promise<void>
}

export const KillSwitchContext = createContext<KillSwitchState | null>(null)

export function useKillSwitch(): KillSwitchState {
  const ctx = useContext(KillSwitchContext)
  if (!ctx) throw new Error('useKillSwitch must be used within KillSwitchProvider')
  return ctx
}
