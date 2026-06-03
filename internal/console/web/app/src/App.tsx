import { useEffect, useState } from 'react'
import { BrowserRouter, Navigate, Route, Routes, useNavigate } from 'react-router-dom'
import { getSetup, setUnauthorizedHandler } from './api'
import { ToastProvider } from './components/Toast'
import { KillSwitchProvider } from './components/KillSwitchContext'
import Layout from './components/Layout'
import Overview from './screens/Overview'
import Events from './screens/Events'
import Sessions from './screens/Sessions'
import Config from './screens/Config'
import Service from './screens/Service'
import Login from './screens/Login'
import Setup from './screens/Setup'

// Installs the 401 → /login redirect for the whole api client.
function UnauthorizedBridge() {
  const navigate = useNavigate()
  useEffect(() => {
    setUnauthorizedHandler(() => {
      if (window.location.pathname !== '/login' && window.location.pathname !== '/setup') {
        navigate('/login')
      }
    })
    return () => setUnauthorizedHandler(null)
  }, [navigate])
  return null
}

// Full-screen boot splash while we determine setup state.
function Boot({ label }: { label: string }) {
  return (
    <div
      className="scanlines"
      style={{ height: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', flexDirection: 'column', gap: '1rem', background: 'var(--color-bg)' }}
    >
      <div className="glow-neon" style={{ fontSize: '1.2rem', letterSpacing: '0.3em', fontWeight: 800 }}>
        ▮▮ PIPELOCK
      </div>
      <div style={{ color: 'var(--color-muted)', fontSize: '0.75rem', letterSpacing: '0.1em' }}>{label}</div>
    </div>
  )
}

// Decides between /setup and the app on first load.
function RootGate() {
  const [state, setState] = useState<'loading' | 'setup' | 'ready' | 'error'>('loading')
  const navigate = useNavigate()

  useEffect(() => {
    let cancelled = false
    getSetup()
      .then((s) => {
        if (cancelled) return
        if (s.needs_setup) {
          setState('setup')
          navigate('/setup', { replace: true })
        } else {
          setState('ready')
        }
      })
      .catch(() => !cancelled && setState('error'))
    return () => {
      cancelled = true
    }
  }, [navigate])

  if (state === 'loading') return <Boot label="initializing console…" />
  if (state === 'error') return <Boot label="cannot reach console backend — retrying may be needed" />
  if (state === 'setup') return <Boot label="first-run setup required…" />
  // ready → render the authenticated app shell. A 401 from the first stats
  // fetch inside it will bounce to /login via the UnauthorizedBridge.
  return (
    <KillSwitchProvider>
      <Layout />
    </KillSwitchProvider>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <ToastProvider>
        <UnauthorizedBridge />
        <Routes>
          <Route path="/setup" element={<Setup />} />
          <Route path="/login" element={<Login />} />
          <Route element={<RootGate />}>
            <Route path="/" element={<Overview />} />
            <Route path="/events" element={<Events />} />
            <Route path="/sessions" element={<Sessions />} />
            <Route path="/config" element={<Config />} />
            <Route path="/service" element={<Service />} />
          </Route>
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </ToastProvider>
    </BrowserRouter>
  )
}
