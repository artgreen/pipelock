import ScreenHeader from '../components/ScreenHeader'

export default function Overview() {
  return (
    <div style={{ padding: '1.5rem 1.75rem' }}>
      <ScreenHeader title="Overview" tag="pipelock" />
      <div className="panel" style={{ color: 'var(--color-muted)' }}>— Overview screen —</div>
    </div>
  )
}
