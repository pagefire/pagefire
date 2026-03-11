import { useState } from 'preact/hooks'
import { useApi } from '../hooks.js'
import { apiPost } from '../api.js'
import { StatusBadge } from '../components/status-badge.jsx'
import { TimeAgo } from '../components/time-ago.jsx'

export function IncidentDetail({ id }) {
  const { data: incident, loading, refetch } = useApi(`/incidents/${id}`)
  const { data: updates, refetch: refetchUpdates } = useApi(`/incidents/${id}/updates`)
  const [message, setMessage] = useState('')
  const [status, setStatus] = useState('')
  const [posting, setPosting] = useState(false)

  const handleAddUpdate = async (e) => {
    e.preventDefault()
    if (!message.trim() || !status) return
    setPosting(true)
    await apiPost(`/incidents/${id}/updates`, { status, message: message.trim() })
    setMessage('')
    setStatus('')
    await refetch()
    await refetchUpdates()
    setPosting(false)
  }

  if (loading) return <div class="loading">Loading...</div>
  if (!incident) return <div class="page"><p>Incident not found</p></div>

  return (
    <div class="page">
      <div class="page-header">
        <div>
          <a href="/incidents" class="breadcrumb">Incidents</a>
          <h1>{incident.title}</h1>
        </div>
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
            <span class="detail-label">Source</span>
            <span>{incident.source}</span>
          </div>
          <div class="detail-row">
            <span class="detail-label">Created</span>
            <TimeAgo time={incident.created_at} />
          </div>
          {incident.summary && (
            <div class="detail-block">
              <span class="detail-label">Summary</span>
              <pre class="detail-pre">{incident.summary}</pre>
            </div>
          )}
        </div>

        <div class="detail-card">
          <h3>Timeline</h3>

          <form class="update-form" onSubmit={handleAddUpdate}>
            <select class="form-select" value={status} onChange={(e) => setStatus(e.target.value)}>
              <option value="">Select status...</option>
              <option value="investigating">Investigating</option>
              <option value="identified">Identified</option>
              <option value="monitoring">Monitoring</option>
              <option value="resolved">Resolved</option>
            </select>
            <input
              class="form-input"
              type="text"
              placeholder="Update message..."
              value={message}
              onInput={(e) => setMessage(e.target.value)}
            />
            <button class="btn btn-primary" type="submit" disabled={posting || !message.trim() || !status}>
              Post
            </button>
          </form>

          {updates && updates.length > 0 ? (
            <div class="timeline">
              {updates.map(u => (
                <div key={u.id} class="timeline-item">
                  <div class="timeline-dot" />
                  <div class="timeline-content">
                    <StatusBadge status={u.status} />
                    <span class="timeline-message">{u.message}</span>
                    <TimeAgo time={u.created_at} />
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <p class="text-muted">No updates yet</p>
          )}
        </div>
      </div>
    </div>
  )
}
