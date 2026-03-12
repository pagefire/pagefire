import { useState, useEffect, useCallback } from 'preact/hooks'
import { apiGet } from '../api.js'
import { StatusBadge } from '../components/status-badge.jsx'
import { TimeAgo } from '../components/time-ago.jsx'

export function Dashboard() {
  const [stats, setStats] = useState(null)
  const [incidents, setIncidents] = useState(null)
  const [alerts, setAlerts] = useState(null)
  const [schedules, setSchedules] = useState(null)
  const [oncall, setOncall] = useState({})
  const [loading, setLoading] = useState(true)

  const fetchData = useCallback(async () => {
    const [triggered, acked, allIncidents, recentAlerts, allSchedules, services] = await Promise.all([
      apiGet('/alerts?status=triggered&limit=1000'),
      apiGet('/alerts?status=acknowledged&limit=1000'),
      apiGet('/incidents?limit=5'),
      apiGet('/alerts?limit=10'),
      apiGet('/schedules'),
      apiGet('/services'),
    ])

    setStats({
      triggered: (triggered.data || []).length,
      acknowledged: (acked.data || []).length,
      incidents: (allIncidents.data || []).filter(i => i.status !== 'resolved').length,
      services: (services.data || []).length,
    })
    setIncidents((allIncidents.data || []).filter(i => i.status !== 'resolved').slice(0, 5))
    setAlerts(recentAlerts.data || [])
    setSchedules(allSchedules.data || [])

    // Fetch on-call for each schedule
    const schedList = allSchedules.data || []
    const oncallResults = {}
    await Promise.all(schedList.map(async (s) => {
      const { data } = await apiGet(`/oncall/${s.id}`)
      oncallResults[s.id] = data || []
    }))
    setOncall(oncallResults)
    setLoading(false)
  }, [])

  useEffect(() => { fetchData() }, [fetchData])

  if (loading) return <div class="loading">Loading...</div>

  return (
    <div class="page">
      <div class="page-header">
        <h1>Dashboard</h1>
      </div>

      <div class="stat-cards">
        <a href="/alerts?status=triggered" class="stat-card stat-card-red">
          <div class="stat-value">{stats.triggered}</div>
          <div class="stat-label">Triggered</div>
        </a>
        <a href="/alerts?status=acknowledged" class="stat-card stat-card-yellow">
          <div class="stat-value">{stats.acknowledged}</div>
          <div class="stat-label">Acknowledged</div>
        </a>
        <div class="stat-card stat-card-orange">
          <div class="stat-value">{stats.incidents}</div>
          <div class="stat-label">Open Incidents</div>
        </div>
        <div class="stat-card stat-card-blue">
          <div class="stat-value">{stats.services}</div>
          <div class="stat-label">Services</div>
        </div>
      </div>

      <div class="dashboard-grid">
        <div class="detail-card">
          <div class="card-header-row">
            <h3>Active Incidents</h3>
            <a href="/incidents" class="btn-link">View all</a>
          </div>
          {incidents.length === 0 ? (
            <p class="text-muted">No active incidents</p>
          ) : (
            <div class="compact-list">
              {incidents.map(inc => (
                <a key={inc.id} href={`/incidents/${inc.id}`} class="compact-list-item">
                  <StatusBadge status={inc.severity} />
                  <span class="compact-title">{inc.title}</span>
                  <StatusBadge status={inc.status} />
                  <TimeAgo time={inc.created_at} />
                </a>
              ))}
            </div>
          )}
        </div>

        <div class="detail-card">
          <div class="card-header-row">
            <h3>On-Call Now</h3>
            <a href="/schedules" class="btn-link">View all</a>
          </div>
          {(!schedules || schedules.length === 0) ? (
            <p class="text-muted">No schedules configured</p>
          ) : (
            <div class="oncall-list">
              {schedules.map(s => {
                const users = oncall[s.id] || []
                return (
                  <a key={s.id} href={`/schedules/${s.id}`} class="oncall-row">
                    <span class="oncall-schedule">{s.name}</span>
                    {users.length > 0 ? (
                      <span class="oncall-user">
                        <span class="oncall-dot" />
                        {users.map(u => u.name).join(', ')}
                      </span>
                    ) : (
                      <span class="text-muted">No one on call</span>
                    )}
                  </a>
                )
              })}
            </div>
          )}
        </div>
      </div>

      <div class="detail-card">
        <div class="card-header-row">
          <h3>Recent Alerts</h3>
          <a href="/alerts" class="btn-link">View all</a>
        </div>
        {alerts.length === 0 ? (
          <p class="text-muted">No alerts</p>
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
    </div>
  )
}
