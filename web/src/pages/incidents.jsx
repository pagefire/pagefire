import { useState } from 'preact/hooks'
import { useApi } from '../hooks.js'
import { apiPost } from '../api.js'
import { StatusBadge } from '../components/status-badge.jsx'
import { TimeAgo } from '../components/time-ago.jsx'
import { EmptyState } from '../components/empty-state.jsx'
import { Modal } from '../components/modal.jsx'
import { TextInput, SelectInput } from '../components/form-field.jsx'
import { useToast } from '../components/toast.jsx'

const SEVERITY_OPTIONS = [
  { value: 'critical', label: 'Critical' },
  { value: 'major', label: 'Major' },
  { value: 'minor', label: 'Minor' },
]

const emptyForm = { title: '', severity: 'major', summary: '' }

export function Incidents() {
  const { data: incidents, loading, refetch } = useApi('/incidents')
  const toast = useToast()

  const [modalOpen, setModalOpen] = useState(false)
  const [form, setForm] = useState(emptyForm)
  const [errors, setErrors] = useState({})
  const [saving, setSaving] = useState(false)

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
    toast.success('Incident created')
    setModalOpen(false)
    setForm(emptyForm)
    refetch()
    if (data?.id) window.location.href = `/incidents/${data.id}`
  }

  const setField = (field) => (e) => {
    setForm(prev => ({ ...prev, [field]: e.target.value }))
    if (errors[field]) setErrors(prev => ({ ...prev, [field]: null }))
  }

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

      {loading ? (
        <div class="loading">Loading...</div>
      ) : !incidents || incidents.length === 0 ? (
        <EmptyState message="No incidents" />
      ) : (
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
            {incidents.map(inc => (
              <tr key={inc.id} class="clickable-row" onClick={() => { window.location.href = `/incidents/${inc.id}` }}>
                <td><StatusBadge status={inc.status} /></td>
                <td><StatusBadge status={inc.severity} /></td>
                <td class="bold">{inc.title}</td>
                <td><TimeAgo time={inc.created_at} /></td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <Modal open={modalOpen} onClose={() => setModalOpen(false)} title="Declare Incident">
        <TextInput label="Title" value={form.title} onInput={setField('title')} error={errors.title} placeholder="API Gateway degraded performance" />
        <SelectInput label="Severity" value={form.severity} onChange={setField('severity')} options={SEVERITY_OPTIONS} />
        <TextInput label="Summary" value={form.summary} onInput={setField('summary')} placeholder="Optional initial summary" />
        <div class="form-actions">
          <button class="btn btn-secondary" onClick={() => setModalOpen(false)}>Cancel</button>
          <button class="btn btn-primary" onClick={handleCreate} disabled={saving}>
            {saving ? 'Creating...' : 'Declare Incident'}
          </button>
        </div>
      </Modal>
    </div>
  )
}
