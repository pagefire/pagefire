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

const PAGE_SIZE = 25

export function Alerts() {
  const [tab, setTab] = useState('triggered')
  const [search, setSearch] = useState('')
  const [debouncedSearch, setDebouncedSearch] = useState('')
  const [alerts, setAlerts] = useState(null)
  const [loading, setLoading] = useState(true)
  const [page, setPage] = useState(0)

  // Debounce search input
  useEffect(() => {
    const timer = setTimeout(() => setDebouncedSearch(search), 300)
    return () => clearTimeout(timer)
  }, [search])

  // Reset page when filters change
  useEffect(() => { setPage(0) }, [tab, debouncedSearch])

  const fetchAlerts = useCallback(async () => {
    setLoading(true)
    const params = new URLSearchParams()
    if (tab) params.set('status', tab)
    if (debouncedSearch) params.set('search', debouncedSearch)
    params.set('limit', PAGE_SIZE + 1) // fetch one extra to detect next page
    params.set('offset', page * PAGE_SIZE)
    const { data } = await apiGet(`/alerts?${params}`)
    setAlerts(data || [])
    setLoading(false)
  }, [tab, debouncedSearch, page])

  useEffect(() => { fetchAlerts() }, [fetchAlerts])

  const hasNext = alerts && alerts.length > PAGE_SIZE
  const displayAlerts = alerts ? alerts.slice(0, PAGE_SIZE) : []

  return (
    <div class="page">
      <div class="page-header">
        <h1>Alerts</h1>
      </div>

      <div class="list-toolbar">
        <div class="tabs">
          {TABS.map(t => (
            <button
              key={t.key}
              class={`tab ${tab === t.key ? 'active' : ''}`}
              onClick={() => setTab(t.key)}
            >
              {t.label}
              {t.key === tab && displayAlerts.length > 0 && (
                <span class="tab-count">{hasNext ? `${PAGE_SIZE}+` : displayAlerts.length}</span>
              )}
            </button>
          ))}
        </div>
        <input
          class="form-input search-input"
          type="text"
          placeholder="Search alerts..."
          value={search}
          onInput={(e) => setSearch(e.target.value)}
        />
      </div>

      {loading ? (
        <div class="loading">Loading...</div>
      ) : displayAlerts.length === 0 ? (
        <EmptyState
          message={debouncedSearch ? 'No matching alerts' : (tab ? `No ${tab} alerts` : 'No alerts')}
          hint={!debouncedSearch && !tab ? 'Alerts are created automatically when monitors or integrations detect issues.' : undefined}
        />
      ) : (
        <>
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
              {displayAlerts.map(a => (
                <tr key={a.id} class="clickable-row" onClick={() => { window.location.href = `/alerts/${a.id}` }}>
                  <td><StatusBadge status={a.status} /></td>
                  <td class="summary-cell">{a.summary}</td>
                  <td><span class="source-tag">{a.source}</span></td>
                  <td><TimeAgo time={a.created_at} /></td>
                </tr>
              ))}
            </tbody>
          </table>
          <div class="pagination">
            <button class="btn btn-secondary btn-sm" disabled={page === 0} onClick={() => setPage(p => p - 1)}>
              Previous
            </button>
            <span class="pagination-info">Page {page + 1}</span>
            <button class="btn btn-secondary btn-sm" disabled={!hasNext} onClick={() => setPage(p => p + 1)}>
              Next
            </button>
          </div>
        </>
      )}
    </div>
  )
}
