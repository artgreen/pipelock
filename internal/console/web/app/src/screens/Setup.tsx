import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { ApiError, postSetup } from '../api'
import AuthShell from '../components/AuthShell'

export default function Setup() {
  const navigate = useNavigate()
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const mismatch = confirm.length > 0 && password !== confirm
  const tooShort = password.length > 0 && password.length < 8

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    if (password !== confirm) {
      setError('Passwords do not match.')
      return
    }
    setBusy(true)
    try {
      await postSetup(password)
      navigate('/', { replace: true })
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setError('Already configured — redirecting to login.')
        setTimeout(() => navigate('/login', { replace: true }), 1200)
      } else {
        setError(err instanceof Error ? err.message : 'Setup failed.')
      }
      setBusy(false)
    }
  }

  return (
    <AuthShell heading="First-run setup" sub="No admin password is configured yet. Set one to secure this console.">
      <form onSubmit={onSubmit}>
        <label style={labelStyle}>admin password</label>
        <input
          className="input-cyber"
          type="password"
          autoFocus
          autoComplete="new-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder="choose a strong password"
        />
        {tooShort && <Hint tone="warn">recommend at least 8 characters</Hint>}

        <label style={{ ...labelStyle, marginTop: '0.9rem' }}>confirm password</label>
        <input
          className="input-cyber"
          type="password"
          autoComplete="new-password"
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
          placeholder="re-enter password"
        />
        {mismatch && <Hint tone="alert">passwords do not match</Hint>}

        {error && (
          <div className="glow-alert" style={{ fontSize: '0.74rem', marginTop: '0.6rem' }}>
            ⚠ {error}
          </div>
        )}
        <button
          type="submit"
          className="btn-neon"
          disabled={busy || !password || mismatch}
          style={{ width: '100%', justifyContent: 'center', marginTop: '1.25rem', opacity: busy || !password || mismatch ? 0.5 : 1 }}
        >
          {busy ? '…configuring' : '▸ set password & enter'}
        </button>
      </form>
    </AuthShell>
  )
}

const labelStyle: React.CSSProperties = {
  display: 'block',
  color: 'var(--color-muted)',
  fontSize: '0.66rem',
  letterSpacing: '0.12em',
  textTransform: 'uppercase',
  marginBottom: '0.4rem',
}

function Hint({ tone, children }: { tone: 'warn' | 'alert'; children: React.ReactNode }) {
  return (
    <div style={{ fontSize: '0.7rem', marginTop: '0.35rem', color: tone === 'alert' ? 'var(--color-alert)' : 'var(--color-warn)' }}>
      {children}
    </div>
  )
}
