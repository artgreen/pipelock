import ScreenHeader from '../components/ScreenHeader'

export default function Service() {
  return (
    <div style={{ padding: '1.5rem 1.75rem' }}>
      <ScreenHeader title="Service" tag="pipelock" />
      <div className="panel" style={{ color: 'var(--color-muted)' }}>— Service screen —</div>
    </div>
  )
}
