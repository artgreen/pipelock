import ScreenHeader from '../components/ScreenHeader'

export default function Sessions() {
  return (
    <div style={{ padding: '1.5rem 1.75rem' }}>
      <ScreenHeader title="Sessions" tag="pipelock" />
      <div className="panel" style={{ color: 'var(--color-muted)' }}>— Sessions screen —</div>
    </div>
  )
}
