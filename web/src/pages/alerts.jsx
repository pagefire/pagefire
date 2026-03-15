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
  const [source, setSource] = useState('')
  const [dateFrom, setDateFrom] = useState('')
  const [dateTo, setDateTo] = useState('')
  const [alerts, setAlerts] = useState(null)
  const [loading, setLoading] = useState(true)
  const [page, setPage] = useState(0)

  // Debounce search input
  useEffect(() => {
    const timer = setTimeout(() => setDebouncedSearch(search), 300)
    return () => clearTimeout(timer)
  }, [search])

  // Reset page when filters change
  useEffect(() => { setPage(0) }, [tab, debouncedSearch, source, dateFrom, dateTo])

  const fetchAlerts = useCallback(async () => {
    setLoading(true)
    const params = new URLSearchParams()
    if (tab) params.set('status', tab)
    if (debouncedSearch) params.set('search', debouncedSearch)
    if (source) params.set('source', source)
    if (dateFrom) params.set('created_after', new Date(dateFrom).toISOString())
    if (dateTo) {
      // Set to end of the selected day
      const end = new Date(dateTo)
      end.setHours(23, 59, 59, 999)
      params.set('created_before', end.toISOString())
    }
    params.set('limit', PAGE_SIZE + 1) // fetch one extra to detect next page
    params.set('offset', page * PAGE_SIZE)
    const { data } = await apiGet(`/alerts?${params}`)
    setAlerts(data || [])
    setLoading(false)
  }, [tab, debouncedSearch, source, dateFrom, dateTo, page])

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

      <div class="list-toolbar" style="gap: 0.5rem; flex-wrap: wrap;">
        <select
          class="form-input"
          value={source}
          onChange={(e) => setSource(e.target.value)}
          style="max-width: 160px;"
        >
          <option value="">All sources</option>
          <option value="api">API</option>
          <option value="email">Email</option>
          <option value="webhook">Webhook</option>
          <option value="monitor">Monitor</option>
        </select>
        <label class="filter-label" style="display: flex; align-items: center; gap: 0.25rem; font-size: 0.85rem; color: var(--text-secondary, #888);">
          From
          <input
            class="form-input"
            type="date"
            value={dateFrom}
            onInput={(e) => setDateFrom(e.target.value)}
            style="max-width: 160px;"
          />
        </label>
        <label class="filter-label" style="display: flex; align-items: center; gap: 0.25rem; font-size: 0.85rem; color: var(--text-secondary, #888);">
          To
          <input
            class="form-input"
            type="date"
            value={dateTo}
            onInput={(e) => setDateTo(e.target.value)}
            style="max-width: 160px;"
          />
        </label>
        {(source || dateFrom || dateTo) && (
          <button
            class="btn btn-secondary btn-sm"
            onClick={() => { setSource(''); setDateFrom(''); setDateTo('') }}
          >
            Clear filters
          </button>
        )}
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
