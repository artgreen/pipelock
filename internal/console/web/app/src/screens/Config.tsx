import ScreenHeader from '../components/ScreenHeader'

export default function Config() {
  return (
    <div style={{ padding: '1.5rem 1.75rem' }}>
      <ScreenHeader title="Config" tag="pipelock" />
      <div className="panel" style={{ color: 'var(--color-muted)' }}>— Config screen —</div>
    </div>
  )
}
