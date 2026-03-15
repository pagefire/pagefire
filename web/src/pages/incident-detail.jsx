import { useState } from 'preact/hooks'
import { useApi } from '../hooks.js'
import { apiPost, apiPut, apiDelete, apiGet } from '../api.js'
import { useAuth } from '../auth.jsx'
import { StatusBadge } from '../components/status-badge.jsx'
import { TimeAgo } from '../components/time-ago.jsx'

const STATUS_FLOW = ['investigating', 'identified', 'monitoring', 'resolved']

export function IncidentDetail({ id }) {
  const { data: incident, loading, refetch } = useApi(`/incidents/${id}`)
  const { data: updates, refetch: refetchUpdates } = useApi(`/incidents/${id}/updates`)
  const { data: linkedAlerts, refetch: refetchAlerts } = useApi(`/incidents/${id}/alerts`)
  const { user } = useAuth()
  const [note, setNote] = useState('')
  const [posting, setPosting] = useState(false)
  const [editingSummary, setEditingSummary] = useState(false)
  const [summaryDraft, setSummaryDraft] = useState('')
  const [editingTitle, setEditingTitle] = useState(false)
  const [titleDraft, setTitleDraft] = useState('')
  const [showLinkModal, setShowLinkModal] = useState(false)
  const [alertSearch, setAlertSearch] = useState('')
  const [alertResults, setAlertResults] = useState([])
  const [searchingAlerts, setSearchingAlerts] = useState(false)

  const changeStatus = async (newStatus) => {
    setPosting(true)
    const label = newStatus.charAt(0).toUpperCase() + newStatus.slice(1)
    await apiPost(`/incidents/${id}/updates`, {
      status: newStatus,
      message: `Status changed to ${label}`,
    })
    await refetch()
    await refetchUpdates()
    setPosting(false)
  }

  const postNote = async (e) => {
    e.preventDefault()
    if (!note.trim()) return
    setPosting(true)
    await apiPost(`/incidents/${id}/updates`, { message: note.trim() })
    setNote('')
    await refetchUpdates()
    setPosting(false)
  }

  const saveSummary = async () => {
    await apiPut(`/incidents/${id}`, {
      ...incident,
      summary: summaryDraft,
    })
    setEditingSummary(false)
    await refetch()
  }

  const startEditSummary = () => {
    setSummaryDraft(incident.summary || '')
    setEditingSummary(true)
  }

  const startEditTitle = () => {
    setTitleDraft(incident.title)
    setEditingTitle(true)
  }

  const saveTitle = async () => {
    if (!titleDraft.trim()) return
    await apiPut(`/incidents/${id}`, {
      ...incident,
      title: titleDraft.trim(),
    })
    setEditingTitle(false)
    await refetch()
  }

  const handleTitleKeyDown = (e) => {
    if (e.key === 'Enter') saveTitle()
    if (e.key === 'Escape') setEditingTitle(false)
  }

  const openLinkModal = async () => {
    setShowLinkModal(true)
    setAlertSearch('')
    setAlertResults([])
    await searchAlerts('')
  }

  const searchAlerts = async (query) => {
    setSearchingAlerts(true)
    const params = query ? `?search=${encodeURIComponent(query)}&limit=20` : '?limit=20'
    const { data } = await apiGet(`/alerts${params}`)
    // Filter out already linked alerts
    const linkedIds = new Set((linkedAlerts || []).map(a => a.id))
    setAlertResults((data || []).filter(a => !linkedIds.has(a.id)))
    setSearchingAlerts(false)
  }

  const handleAlertSearchInput = async (e) => {
    const val = e.target.value
    setAlertSearch(val)
    await searchAlerts(val)
  }

  const linkAlert = async (alertID) => {
    await apiPost(`/incidents/${id}/alerts`, { alert_id: alertID })
    await refetchAlerts()
    // Remove from search results
    setAlertResults(prev => prev.filter(a => a.id !== alertID))
  }

  const unlinkAlert = async (alertID) => {
    await apiDelete(`/incidents/${id}/alerts/${alertID}`)
    await refetchAlerts()
  }

  if (loading) return <div class="loading">Loading...</div>
  if (!incident) return <div class="page"><p>Incident not found</p></div>

  const currentIdx = STATUS_FLOW.indexOf(incident.status)
  const nextStatuses = STATUS_FLOW.filter((_, i) => i > currentIdx)
  const isResolved = incident.status === 'resolved'

  return (
    <div class="page">
      <div class="page-header">
        <div>
          <a href="/incidents" class="breadcrumb">Incidents</a>
          {editingTitle ? (
            <div class="title-edit">
              <input
                class="form-input title-input"
                type="text"
                value={titleDraft}
                onInput={(e) => setTitleDraft(e.target.value)}
                onKeyDown={handleTitleKeyDown}
                autoFocus
              />
              <button class="btn btn-primary btn-sm" onClick={saveTitle}>Save</button>
              <button class="btn btn-secondary btn-sm" onClick={() => setEditingTitle(false)}>Cancel</button>
            </div>
          ) : (
            <h1 class="editable-title" onClick={startEditTitle}>{incident.title}</h1>
          )}
        </div>
        {!isResolved && (
          <div class="actions">
            {nextStatuses.map(s => (
              <button
                key={s}
                class={`btn ${s === 'resolved' ? 'btn-success' : 'btn-secondary'}`}
                onClick={() => changeStatus(s)}
                disabled={posting}
              >
                {s === 'resolved' ? 'Resolve' : `Mark ${s.charAt(0).toUpperCase() + s.slice(1)}`}
              </button>
            ))}
          </div>
        )}
      </div>

      <div class="detail-grid">
        <div class="detail-card">
          <h3>Details</h3>
          <div class="detail-row">
            <span class="detail-label">Status</span>
            <StatusBadge status={incident.status} />
          </div>
          <div class="detail-row">
            <span class="detail-label">Severity</span>
            <StatusBadge status={incident.severity} />
          </div>
          <div class="detail-row">
            <span class="detail-label">Declared</span>
            <TimeAgo time={incident.created_at} />
          </div>
          {incident.resolved_at && (
            <div class="detail-row">
              <span class="detail-label">Resolved</span>
              <TimeAgo time={incident.resolved_at} />
            </div>
          )}

          <div class="detail-block">
            <div class="detail-label-row">
              <span class="detail-label">Summary</span>
              {!editingSummary && (
                <button class="btn-link" onClick={startEditSummary}>Edit</button>
              )}
            </div>
            {editingSummary ? (
              <div class="summary-edit">
                <textarea
                  class="form-control"
                  rows={4}
                  value={summaryDraft}
                  onInput={(e) => setSummaryDraft(e.target.value)}
                  placeholder="Describe what happened and the impact..."
                />
                <div class="summary-edit-actions">
                  <button class="btn btn-secondary btn-sm" onClick={() => setEditingSummary(false)}>Cancel</button>
                  <button class="btn btn-primary btn-sm" onClick={saveSummary}>Save</button>
                </div>
              </div>
            ) : (
              <pre class="detail-pre">{incident.summary || 'No summary yet'}</pre>
            )}
          </div>
        </div>

        <div class="detail-card">
          <div class="detail-label-row">
            <h3>Alerts</h3>
            <button class="btn btn-secondary btn-sm" onClick={openLinkModal}>Link Alert</button>
          </div>

          {linkedAlerts && linkedAlerts.length > 0 ? (
            <div class="linked-alerts-list">
              {linkedAlerts.map(alert => (
                <div key={alert.id} class="linked-alert-item">
                  <div class="linked-alert-info">
                    <a href={`/alerts/${alert.id}`} class="linked-alert-summary">{alert.summary}</a>
                    <div class="linked-alert-meta">
                      <StatusBadge status={alert.status} />
                      <TimeAgo time={alert.created_at} />
                    </div>
                  </div>
                  <button
                    class="btn-icon btn-unlink"
                    onClick={() => unlinkAlert(alert.id)}
                    title="Unlink alert"
                  >
                    &times;
                  </button>
                </div>
              ))}
            </div>
          ) : (
            <p class="text-muted">No alerts linked</p>
          )}

          {showLinkModal && (
            <div class="modal-overlay" onClick={() => setShowLinkModal(false)}>
              <div class="modal" onClick={(e) => e.stopPropagation()}>
                <div class="modal-header">
                  <h3>Link Alert</h3>
                  <button class="btn-icon" onClick={() => setShowLinkModal(false)}>&times;</button>
                </div>
                <div class="modal-body">
                  <input
                    class="form-input"
                    type="text"
                    placeholder="Search alerts..."
                    value={alertSearch}
                    onInput={handleAlertSearchInput}
                    autoFocus
                  />
                  <div class="alert-search-results">
                    {searchingAlerts ? (
                      <p class="text-muted">Searching...</p>
                    ) : alertResults.length > 0 ? (
                      alertResults.map(alert => (
                        <div key={alert.id} class="alert-search-item" onClick={() => linkAlert(alert.id)}>
                          <div class="alert-search-info">
                            <span class="alert-search-summary">{alert.summary}</span>
                            <div class="alert-search-meta">
                              <StatusBadge status={alert.status} />
                              <TimeAgo time={alert.created_at} />
                            </div>
                          </div>
                        </div>
                      ))
                    ) : (
                      <p class="text-muted">No alerts found</p>
                    )}
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>

        <div class="detail-card">
          <h3>Correspondence</h3>

          {!isResolved && (
            <form class="note-form" onSubmit={postNote}>
              <input
                class="form-input"
                type="text"
                placeholder="Add a note..."
                value={note}
                onInput={(e) => setNote(e.target.value)}
              />
              <button class="btn btn-primary" type="submit" disabled={posting || !note.trim()}>
                Post
              </button>
            </form>
          )}

          {updates && updates.length > 0 ? (
            <div class="timeline">
              {[...updates].reverse().map(u => (
                <div key={u.id} class={`timeline-item ${u.status ? 'timeline-status' : 'timeline-note'}`}>
                  <div class="timeline-dot" />
                  <div class="timeline-content">
                    <div class="timeline-header">
                      {u.created_by_name && <span class="timeline-author">{u.created_by_name}</span>}
                      {u.status && <StatusBadge status={u.status} />}
                      <TimeAgo time={u.created_at} />
                    </div>
                    <span class="timeline-message">{u.message}</span>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <p class="text-muted">No activity yet</p>
          )}
        </div>
      </div>
    </div>
  )
}
