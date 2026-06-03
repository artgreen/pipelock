import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { ApiError, login } from '../api'
import AuthShell from '../components/AuthShell'

export default function Login() {
  const navigate = useNavigate()
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    setBusy(true)
    try {
      await login(password)
      navigate('/', { replace: true })
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setError('Incorrect password.')
      } else {
        setError(err instanceof Error ? err.message : 'Login failed.')
      }
      setBusy(false)
    }
  }

  return (
    <AuthShell heading="Authenticate" sub="Enter the admin password to open the egress monitor.">
      <form onSubmit={onSubmit}>
        <label style={{ display: 'block', color: 'var(--color-muted)', fontSize: '0.66rem', letterSpacing: '0.12em', textTransform: 'uppercase', marginBottom: '0.4rem' }}>
          password
        </label>
        <input
          className="input-cyber"
          type="password"
          autoFocus
          autoComplete="current-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder="••••••••"
        />
        {error && (
          <div className="glow-alert" style={{ fontSize: '0.74rem', marginTop: '0.6rem' }}>
            ⚠ {error}
          </div>
        )}
        <button type="submit" className="btn-neon" disabled={busy || !password} style={{ width: '100%', justifyContent: 'center', marginTop: '1.25rem', opacity: busy || !password ? 0.5 : 1 }}>
          {busy ? '…authenticating' : '▸ enter console'}
        </button>
      </form>
    </AuthShell>
  )
}
