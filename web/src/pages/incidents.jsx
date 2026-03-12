import { useState, useEffect, useCallback } from 'preact/hooks'
import { useApi } from '../hooks.js'
import { apiGet, apiPost } from '../api.js'
import { StatusBadge } from '../components/status-badge.jsx'
import { TimeAgo } from '../components/time-ago.jsx'
import { EmptyState } from '../components/empty-state.jsx'
import { Modal } from '../components/modal.jsx'
import { TextInput, TextArea, SelectInput } from '../components/form-field.jsx'
import { useToast } from '../components/toast.jsx'

const SEVERITY_OPTIONS = [
  { value: 'critical', label: 'Critical' },
  { value: 'major', label: 'Major' },
  { value: 'minor', label: 'Minor' },
]

const STATUS_TABS = [
  { key: '', label: 'All' },
  { key: 'investigating', label: 'Investigating' },
  { key: 'identified', label: 'Identified' },
  { key: 'monitoring', label: 'Monitoring' },
  { key: 'resolved', label: 'Resolved' },
]

const PAGE_SIZE = 25
const emptyForm = { title: '', severity: 'major', summary: '', service_id: '' }

export function Incidents() {
  const [tab, setTab] = useState('')
  const [search, setSearch] = useState('')
  const [debouncedSearch, setDebouncedSearch] = useState('')
  const [incidents, setIncidents] = useState(null)
  const [loading, setLoading] = useState(true)
  const [page, setPage] = useState(0)

  const { data: services } = useApi('/services')
  const toast = useToast()

  const [modalOpen, setModalOpen] = useState(false)
  const [form, setForm] = useState(emptyForm)
  const [errors, setErrors] = useState({})
  const [saving, setSaving] = useState(false)

  // Debounce search
  useEffect(() => {
    const timer = setTimeout(() => setDebouncedSearch(search), 300)
    return () => clearTimeout(timer)
  }, [search])

  useEffect(() => { setPage(0) }, [tab, debouncedSearch])

  const fetchIncidents = useCallback(async () => {
    setLoading(true)
    const params = new URLSearchParams()
    if (tab) params.set('status', tab)
    if (debouncedSearch) params.set('search', debouncedSearch)
    params.set('limit', PAGE_SIZE + 1)
    params.set('offset', page * PAGE_SIZE)
    const { data } = await apiGet(`/incidents?${params}`)
    setIncidents(data || [])
    setLoading(false)
  }, [tab, debouncedSearch, page])

  useEffect(() => { fetchIncidents() }, [fetchIncidents])

  const handleCreate = async () => {
    const errs = {}
    if (!form.title.trim()) errs.title = 'Title is required'
    setErrors(errs)
    if (Object.keys(errs).length > 0) return

    setSaving(true)
    const { data, error } = await apiPost('/incidents', {
      title: form.title.trim(),
      severity: form.severity,
      summary: form.summary.trim(),
      status: 'investigating',
    })
    setSaving(false)
    if (error) {
      toast.error(error)
      return
    }

    if (form.service_id && data?.id) {
      await apiPost(`/incidents/${data.id}/services`, { service_id: form.service_id })
    }

    toast.success('Incident declared')
    setModalOpen(false)
    setForm(emptyForm)
    fetchIncidents()
    if (data?.id) window.location.href = `/incidents/${data.id}`
  }

  const setField = (field) => (e) => {
    setForm(prev => ({ ...prev, [field]: e.target.value }))
    if (errors[field]) setErrors(prev => ({ ...prev, [field]: null }))
  }

  const serviceOptions = (services || []).map(s => ({ value: s.id, label: s.name }))
  const hasNext = incidents && incidents.length > PAGE_SIZE
  const displayIncidents = incidents ? incidents.slice(0, PAGE_SIZE) : []

  return (
    <div class="page">
      <div class="page-header">
        <h1>Incidents</h1>
        <div class="actions">
          <button class="btn btn-primary" onClick={() => { setModalOpen(true); setForm(emptyForm); setErrors({}) }}>
            Declare Incident
          </button>
        </div>
      </div>

      <div class="list-toolbar">
        <div class="tabs">
          {STATUS_TABS.map(t => (
            <button
              key={t.key}
              class={`tab ${tab === t.key ? 'active' : ''}`}
              onClick={() => setTab(t.key)}
            >
              {t.label}
            </button>
          ))}
        </div>
        <input
          class="form-input search-input"
          type="text"
          placeholder="Search incidents..."
          value={search}
          onInput={(e) => setSearch(e.target.value)}
        />
      </div>

      {loading ? (
        <div class="loading">Loading...</div>
      ) : displayIncidents.length === 0 ? (
        <EmptyState message={debouncedSearch ? 'No matching incidents' : 'No incidents'} />
      ) : (
        <>
          <table class="data-table">
            <thead>
              <tr>
                <th>Status</th>
                <th>Severity</th>
                <th>Title</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              {displayIncidents.map(inc => (
                <tr key={inc.id} class="clickable-row" onClick={() => { window.location.href = `/incidents/${inc.id}` }}>
                  <td><StatusBadge status={inc.status} /></td>
                  <td><StatusBadge status={inc.severity} /></td>
                  <td class="bold">{inc.title}</td>
                  <td><TimeAgo time={inc.created_at} /></td>
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

      <Modal open={modalOpen} onClose={() => setModalOpen(false)} title="Declare Incident">
        <TextInput label="Title" value={form.title} onInput={setField('title')} error={errors.title} placeholder="API Gateway degraded performance" />
        <SelectInput label="Severity" value={form.severity} onChange={setField('severity')} options={SEVERITY_OPTIONS} />
        <TextArea label="Summary" value={form.summary} onInput={setField('summary')} placeholder="Describe what's happening and the impact..." />
        {serviceOptions.length > 0 && (
          <SelectInput label="Related Service" value={form.service_id} onChange={setField('service_id')} options={serviceOptions} placeholder="None" />
        )}
        <div class="form-actions">
          <button class="btn btn-secondary" onClick={() => setModalOpen(false)}>Cancel</button>
          <button class="btn btn-primary" onClick={handleCreate} disabled={saving}>
            {saving ? 'Declaring...' : 'Declare Incident'}
          </button>
        </div>
      </Modal>
    </div>
  )
}
