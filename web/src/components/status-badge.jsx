const STATUS_COLORS = {
  triggered: 'badge-red',
  acknowledged: 'badge-yellow',
  resolved: 'badge-green',
  investigating: 'badge-yellow',
  identified: 'badge-yellow',
  monitoring: 'badge-blue',
  critical: 'badge-red',
  major: 'badge-orange',
  minor: 'badge-yellow',
}

export function StatusBadge({ status }) {
  const colorClass = STATUS_COLORS[status] || 'badge-gray'
  return <span class={`badge ${colorClass}`}>{status}</span>
}
