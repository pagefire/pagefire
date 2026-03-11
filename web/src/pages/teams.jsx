import { useState } from 'preact/hooks'
import { useApi } from '../hooks.js'
import { useAuth } from '../auth.jsx'
import { apiPost, apiPut, apiDelete } from '../api.js'
import { EmptyState } from '../components/empty-state.jsx'
import { Modal } from '../components/modal.jsx'
import { TextInput, SelectInput } from '../components/form-field.jsx'
import { ConfirmDialog } from '../components/confirm-dialog.jsx'
import { useToast } from '../components/toast.jsx'

const emptyForm = { name: '', description: '' }

export function Teams() {
  const { data: teams, loading, refetch } = useApi('/teams')
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

  const openEdit = (team, e) => {
    e.stopPropagation()
    setEditing(team)
    setForm({ name: team.name, description: team.description || '' })
    setErrors({})
    setModalOpen(true)
  }

  const handleSave = async () => {
    if (!form.name.trim()) {
      setErrors({ name: 'Name is required' })
      return
    }
    setSaving(true)
    const payload = { name: form.name.trim(), description: form.description.trim() }
    const { error } = editing
      ? await apiPut(`/teams/${editing.id}`, payload)
      : await apiPost('/teams', payload)
    setSaving(false)
    if (error) {
      toast.error(error)
      return
    }
    toast.success(editing ? 'Team updated' : 'Team created')
    setModalOpen(false)
    refetch()
  }

  const handleDelete = async () => {
    const { error } = await apiDelete(`/teams/${deleteTarget.id}`)
    if (error) {
      toast.error(error)
    } else {
      toast.success('Team deleted')
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
        <h1>Teams</h1>
        <div class="actions">
          {isAdmin && <button class="btn btn-primary" onClick={openCreate}>Add Team</button>}
        </div>
      </div>

      {loading ? (
        <div class="loading">Loading...</div>
      ) : !teams || teams.length === 0 ? (
        <EmptyState message="No teams configured" />
      ) : (
        <table class="data-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Description</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {teams.map(t => (
              <tr key={t.id} class="clickable-row" onClick={() => { window.location.href = `/teams/${t.id}` }}>
                <td class="bold">{t.name}</td>
                <td class="text-muted">{t.description || '—'}</td>
                {isAdmin && (
                  <td class="row-actions">
                    <button class="btn-icon" onClick={(e) => openEdit(t, e)} title="Edit">&#9998;</button>
                    <button class="btn-icon btn-icon-danger" onClick={(e) => { e.stopPropagation(); setDeleteTarget(t) }} title="Delete">&times;</button>
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <Modal open={modalOpen} onClose={() => setModalOpen(false)} title={editing ? 'Edit Team' : 'Add Team'}>
        <TextInput label="Name" value={form.name} onInput={setField('name')} error={errors.name} placeholder="Platform Engineering" />
        <TextInput label="Description" value={form.description} onInput={setField('description')} placeholder="Optional description" />
        <div class="form-actions">
          <button class="btn btn-secondary" onClick={() => setModalOpen(false)}>Cancel</button>
          <button class="btn btn-primary" onClick={handleSave} disabled={saving}>
            {saving ? 'Saving...' : editing ? 'Save Changes' : 'Create Team'}
          </button>
        </div>
      </Modal>

      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDelete}
        title="Delete Team"
        message={`Are you sure you want to delete "${deleteTarget?.name}"?`}
      />
    </div>
  )
}
