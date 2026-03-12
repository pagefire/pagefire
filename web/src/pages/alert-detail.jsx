import { useState } from 'preact/hooks'
import { useApi } from '../hooks.js'
import { apiPost } from '../api.js'
import { useAuth } from '../auth.jsx'
import { StatusBadge } from '../components/status-badge.jsx'
import { TimeAgo } from '../components/time-ago.jsx'

export function AlertDetail({ id }) {
  const { data: alert, loading, refetch } = useApi(`/alerts/${id}`)
  const { data: logs, refetch: refetchLogs } = useApi(`/alerts/${id}/logs`)
  const { data: services } = useApi('/services')
  const { user: currentUser } = useAuth()
  const [acting, setActing] = useState(false)
  const [actionError, setActionError] = useState(null)

  const handleAction = async (action) => {
    setActing(true)
    const { error } = await apiPost(`/alerts/${id}/${action}`, { user_id: currentUser?.id || '' })
    if (error) {
      setActionError(`Failed to ${action}: ${error}`)
    } else {
      setActionError(null)
      await refetch()
      await refetchLogs()
    }
    setActing(false)
  }

  if (loading) return <div class="loading">Loading...</div>
  if (!alert) return <div class="page"><p>Alert not found</p></div>

  return (
    <div class="page">
      <div class="page-header">
        <div>
          <a href="/alerts" class="breadcrumb">Alerts</a>
          <h1>{alert.summary}</h1>
        </div>
        <div class="actions">
          {alert.status === 'triggered' && (
            <button class="btn btn-warning" onClick={() => handleAction('acknowledge')} disabled={acting}>
              Acknowledge
            </button>
          )}
          {alert.status !== 'resolved' && (
            <button class="btn btn-success" onClick={() => handleAction('resolve')} disabled={acting}>
              Resolve
            </button>
          )}
        </div>
      </div>

      {actionError && <div class="alert alert-error">{actionError}</div>}

      <div class="detail-grid">
        <div class="detail-card">
          <h3>Details</h3>
          <div class="detail-row">
            <span class="detail-label">Status</span>
            <StatusBadge status={alert.status} />
          </div>
          <div class="detail-row">
            <span class="detail-label">Source</span>
            <span>{alert.source}</span>
          </div>
          <div class="detail-row">
            <span class="detail-label">Service</span>
            <span>{(services || []).find(s => s.id === alert.service_id)?.name || alert.service_id}</span>
          </div>
          {alert.dedup_key && (
            <div class="detail-row">
              <span class="detail-label">Dedup Key</span>
              <span class="mono">{alert.dedup_key}</span>
            </div>
          )}
          {alert.group_key && (
            <div class="detail-row">
              <span class="detail-label">Group Key</span>
              <span class="mono">{alert.group_key}</span>
            </div>
          )}
          <div class="detail-row">
            <span class="detail-label">Created</span>
            <TimeAgo time={alert.created_at} />
          </div>
          {alert.acknowledged_at && (
            <div class="detail-row">
              <span class="detail-label">Acknowledged</span>
              <TimeAgo time={alert.acknowledged_at} />
            </div>
          )}
          {alert.resolved_at && (
            <div class="detail-row">
              <span class="detail-label">Resolved</span>
              <TimeAgo time={alert.resolved_at} />
            </div>
          )}
          {alert.details && (
            <div class="detail-block">
              <span class="detail-label">Details</span>
              <pre class="detail-pre">{alert.details}</pre>
            </div>
          )}
        </div>

        <div class="detail-card">
          <h3>Activity Log</h3>
          {logs && logs.length > 0 ? (
            <div class="timeline">
              {logs.map(log => (
                <div key={log.id} class="timeline-item">
                  <div class="timeline-dot" />
                  <div class="timeline-content">
                    <span class="timeline-event">{log.event}</span>
                    <span class="timeline-message">{log.message}</span>
                    <TimeAgo time={log.created_at} />
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <p class="text-muted">No log entries</p>
          )}
        </div>
      </div>
    </div>
  )
}
