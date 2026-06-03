import { createContext, useContext } from 'react'

export type ToastTone = 'ok' | 'alert' | 'warn'

export interface ToastCtx {
  push: (message: string, tone?: ToastTone) => void
}

export const ToastContext = createContext<ToastCtx | null>(null)

export function useToast(): ToastCtx {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used within ToastProvider')
  return ctx
}
