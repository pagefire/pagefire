import { useState } from 'preact/hooks'
import { useApi } from '../hooks.js'
import { useAuth } from '../auth.jsx'
import { apiPost, apiPut, apiDelete } from '../api.js'
import { EmptyState } from '../components/empty-state.jsx'
import { Modal } from '../components/modal.jsx'
import { TextInput, SelectInput } from '../components/form-field.jsx'

const TIMEZONES = [
  'UTC', 'America/New_York', 'America/Chicago', 'America/Denver',
  'America/Los_Angeles', 'America/Sao_Paulo', 'Europe/London',
  'Europe/Berlin', 'Europe/Moscow', 'Asia/Kolkata', 'Asia/Shanghai',
  'Asia/Tokyo', 'Australia/Sydney', 'Pacific/Auckland',
].map(tz => ({ value: tz, label: tz }))
import { ConfirmDialog } from '../components/confirm-dialog.jsx'
import { useToast } from '../components/toast.jsx'

const emptyForm = { name: '', description: '', timezone: 'UTC' }

export function Schedules() {
  const { data: schedules, loading, refetch } = useApi('/schedules')
  const { user: currentUser } = useAuth()
  const isAdmin = currentUser?.role === 'admin'
  const toast = useToast()

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState(null)
  const [form, setForm] = useState(emptyForm)
  const [errors, setErrors] = useState({})
  const [saving, setSaving] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState(null)

  const openCreate = () => {
    setEditing(null)
    setForm(emptyForm)
    setErrors({})
    setModalOpen(true)
  }

  const openEdit = (schedule, e) => {
    e.stopPropagation()
    setEditing(schedule)
    setForm({ name: schedule.name, description: schedule.description || '', timezone: schedule.timezone || 'UTC' })
    setErrors({})
    setModalOpen(true)
  }

  const handleSave = async () => {
    if (!form.name.trim()) {
      setErrors({ name: 'Name is required' })
      return
    }
    setSaving(true)
    const payload = { name: form.name.trim(), description: form.description.trim(), timezone: form.timezone.trim() || 'UTC' }
    const { error } = editing
      ? await apiPut(`/schedules/${editing.id}`, payload)
      : await apiPost('/schedules', payload)
    setSaving(false)
    if (error) {
      toast.error(error)
      return
    }
    toast.success(editing ? 'Schedule updated' : 'Schedule created')
    setModalOpen(false)
    refetch()
  }

  const handleDelete = async () => {
    const { error } = await apiDelete(`/schedules/${deleteTarget.id}`)
    if (error) {
      toast.error(error)
    } else {
      toast.success('Schedule deleted')
      refetch()
    }
    setDeleteTarget(null)
  }

  const setField = (field) => (e) => {
    setForm(prev => ({ ...prev, [field]: e.target.value }))
    if (errors[field]) setErrors(prev => ({ ...prev, [field]: null }))
  }

  return (
    <div class="page">
      <div class="page-header">
        <h1>Schedules</h1>
        <div class="actions">
          {isAdmin && <button class="btn btn-primary" onClick={openCreate}>Add Schedule</button>}
        </div>
      </div>

      {loading ? (
        <div class="loading">Loading...</div>
      ) : !schedules || schedules.length === 0 ? (
        <EmptyState message="No schedules configured" />
      ) : (
        <table class="data-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Timezone</th>
              <th>Description</th>
              {isAdmin && <th></th>}
            </tr>
          </thead>
          <tbody>
            {schedules.map(s => (
              <tr key={s.id} class="clickable-row" onClick={() => { window.location.href = `/schedules/${s.id}` }}>
                <td class="bold">{s.name}</td>
                <td>{s.timezone}</td>
                <td class="text-muted">{s.description || '—'}</td>
                {isAdmin && (
                  <td class="row-actions">
                    <button class="btn-icon" onClick={(e) => openEdit(s, e)} title="Edit">&#9998;</button>
                    <button class="btn-icon btn-icon-danger" onClick={(e) => { e.stopPropagation(); setDeleteTarget(s) }} title="Delete">&times;</button>
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <Modal open={modalOpen} onClose={() => setModalOpen(false)} title={editing ? 'Edit Schedule' : 'Add Schedule'}>
        <TextInput label="Name" value={form.name} onInput={setField('name')} error={errors.name} placeholder="Primary On-Call" />
        <SelectInput label="Timezone" value={form.timezone} onChange={setField('timezone')} options={TIMEZONES} />
        <TextInput label="Description" value={form.description} onInput={setField('description')} placeholder="Optional description" />
        <div class="form-actions">
          <button class="btn btn-secondary" onClick={() => setModalOpen(false)}>Cancel</button>
          <button class="btn btn-primary" onClick={handleSave} disabled={saving}>
            {saving ? 'Saving...' : editing ? 'Save Changes' : 'Create Schedule'}
          </button>
        </div>
      </Modal>

      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDelete}
        title="Delete Schedule"
        message={`Are you sure you want to delete "${deleteTarget?.name}"?`}
      />
    </div>
  )
}
