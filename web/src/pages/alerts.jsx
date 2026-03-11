import { useState, useEffect, useCallback } from 'preact/hooks'
import { apiGet } from '../api.js'
import { StatusBadge } from '../components/status-badge.jsx'
import { TimeAgo } from '../components/time-ago.jsx'
import { EmptyState } from '../components/empty-state.jsx'

const TABS = [
  { key: '', label: 'All' },
  { key: 'triggered', label: 'Triggered' },
  { key: 'acknowledged', label: 'Acknowledged' },
  { key: 'resolved', label: 'Resolved' },
]

export function Alerts() {
  const [tab, setTab] = useState('triggered')
  const [alerts, setAlerts] = useState(null)
  const [loading, setLoading] = useState(true)

  const fetchAlerts = useCallback(async () => {
    setLoading(true)
    const query = tab ? `?status=${tab}` : ''
    const { data } = await apiGet(`/alerts${query}`)
    setAlerts(data || [])
    setLoading(false)
  }, [tab])

  useEffect(() => { fetchAlerts() }, [fetchAlerts])

  return (
    <div class="page">
      <div class="page-header">
        <h1>Alerts</h1>
      </div>

      <div class="tabs">
        {TABS.map(t => (
          <button
            key={t.key}
            class={`tab ${tab === t.key ? 'active' : ''}`}
            onClick={() => setTab(t.key)}
          >
            {t.label}
            {alerts && t.key === tab && <span class="tab-count">{alerts.length}</span>}
          </button>
        ))}
      </div>

      {loading ? (
        <div class="loading">Loading...</div>
      ) : alerts.length === 0 ? (
        <EmptyState message={tab ? `No ${tab} alerts` : 'No alerts'} />
      ) : (
        <table class="data-table">
          <thead>
            <tr>
              <th>Status</th>
              <th>Summary</th>
              <th>Source</th>
              <th>Created</th>
            </tr>
          </thead>
          <tbody>
            {alerts.map(a => (
              <tr key={a.id} class="clickable-row" onClick={() => { window.location.href = `/alerts/${a.id}` }}>
                <td><StatusBadge status={a.status} /></td>
                <td class="summary-cell">{a.summary}</td>
                <td><span class="source-tag">{a.source}</span></td>
                <td><TimeAgo time={a.created_at} /></td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
