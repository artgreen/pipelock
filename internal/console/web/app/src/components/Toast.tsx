import { createContext, useCallback, useContext, useState, type ReactNode } from 'react'

type ToastTone = 'ok' | 'alert' | 'warn'

interface Toast {
  id: number
  message: string
  tone: ToastTone
}

interface ToastCtx {
  push: (message: string, tone?: ToastTone) => void
}

const Ctx = createContext<ToastCtx | null>(null)

export function useToast(): ToastCtx {
  const ctx = useContext(Ctx)
  if (!ctx) throw new Error('useToast must be used within ToastProvider')
  return ctx
}

let nextId = 1

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])

  const push = useCallback((message: string, tone: ToastTone = 'ok') => {
    const id = nextId++
    setToasts((t) => [...t, { id, message, tone }])
    window.setTimeout(() => {
      setToasts((t) => t.filter((x) => x.id !== id))
    }, 5000)
  }, [])

  const dismiss = (id: number) => setToasts((t) => t.filter((x) => x.id !== id))

  return (
    <Ctx.Provider value={{ push }}>
      {children}
      <div
        style={{
          position: 'fixed',
          bottom: '1.25rem',
          right: '1.25rem',
          zIndex: 400,
          display: 'flex',
          flexDirection: 'column',
          gap: '0.5rem',
          maxWidth: '380px',
        }}
      >
        {toasts.map((t) => (
          <div
            key={t.id}
            onClick={() => dismiss(t.id)}
            className="panel"
            style={{
              cursor: 'pointer',
              borderColor:
                t.tone === 'alert'
                  ? 'var(--color-alert)'
                  : t.tone === 'warn'
                    ? '#5a5000'
                    : 'var(--color-neon-dim)',
              color:
                t.tone === 'alert'
                  ? 'var(--color-alert)'
                  : t.tone === 'warn'
                    ? 'var(--color-warn)'
                    : 'var(--color-neon)',
              fontSize: '0.78rem',
              padding: '0.7rem 0.9rem',
              boxShadow: '0 4px 18px rgba(0,0,0,0.5)',
              animation: 'toastIn 0.2s ease',
              display: 'flex',
              gap: '0.5rem',
            }}
          >
            <span aria-hidden>{t.tone === 'alert' ? '⚠' : t.tone === 'warn' ? '▲' : '✓'}</span>
            <span style={{ color: 'var(--color-text)' }}>{t.message}</span>
          </div>
        ))}
      </div>
      <style>{`@keyframes toastIn { from { opacity: 0; transform: translateY(8px); } to { opacity: 1; transform: translateY(0); } }`}</style>
    </Ctx.Provider>
  )
}
