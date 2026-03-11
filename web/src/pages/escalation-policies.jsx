import { useState } from 'preact/hooks'
import { useApi } from '../hooks.js'
import { useAuth } from '../auth.jsx'

import { apiPost, apiPut, apiDelete } from '../api.js'
import { EmptyState } from '../components/empty-state.jsx'
import { Modal } from '../components/modal.jsx'
import { TextInput, SelectInput } from '../components/form-field.jsx'
import { ConfirmDialog } from '../components/confirm-dialog.jsx'
import { useToast } from '../components/toast.jsx'

const REPEAT_OPTIONS = [0, 1, 2, 3, 4, 5].map(n => ({ value: String(n), label: `${n}x` }))

const emptyForm = { name: '', description: '', repeat: '0' }

export function EscalationPolicies() {
  const { data: policies, loading, refetch } = useApi('/escalation-policies')
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

  const openEdit = (policy, e) => {
    e.stopPropagation()
    setEditing(policy)
    setForm({ name: policy.name, description: policy.description || '', repeat: String(policy.repeat || 0) })
    setErrors({})
    setModalOpen(true)
  }

  const validate = () => {
    const errs = {}
    if (!form.name.trim()) errs.name = 'Name is required'
    setErrors(errs)
    return Object.keys(errs).length === 0
  }

  const handleSave = async () => {
    if (!validate()) return
    setSaving(true)
    const payload = { name: form.name.trim(), description: form.description.trim(), repeat: parseInt(form.repeat, 10) }
    const { error } = editing
      ? await apiPut(`/escalation-policies/${editing.id}`, payload)
      : await apiPost('/escalation-policies', payload)
    setSaving(false)
    if (error) {
      toast.error(error)
      return
    }
    toast.success(editing ? 'Policy updated' : 'Policy created')
    setModalOpen(false)
    refetch()
  }

  const handleDelete = async () => {
    const { error } = await apiDelete(`/escalation-policies/${deleteTarget.id}`)
    if (error) {
      toast.error(error)
    } else {
      toast.success('Policy deleted')
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
        <h1>Escalation Policies</h1>
        <div class="actions">
          {isAdmin && <button class="btn btn-primary" onClick={openCreate}>Add Policy</button>}
        </div>
      </div>

      {loading ? (
        <div class="loading">Loading...</div>
      ) : !policies || policies.length === 0 ? (
        <EmptyState message="No escalation policies configured" />
      ) : (
        <table class="data-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Description</th>
              <th>Repeat</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {policies.map(p => (
              <tr key={p.id} class="clickable-row" onClick={() => { window.location.href = `/escalation-policies/${p.id}` }}>
                <td class="bold">{p.name}</td>
                <td class="text-muted">{p.description || '—'}</td>
                <td>{p.repeat}x</td>
                {isAdmin && (
                  <td class="row-actions">
                    <button class="btn-icon" onClick={(e) => openEdit(p, e)} title="Edit">&#9998;</button>
                    <button class="btn-icon btn-icon-danger" onClick={(e) => { e.stopPropagation(); setDeleteTarget(p) }} title="Delete">&times;</button>
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <Modal open={modalOpen} onClose={() => setModalOpen(false)} title={editing ? 'Edit Policy' : 'Add Escalation Policy'}>
        <TextInput label="Name" value={form.name} onInput={setField('name')} error={errors.name} placeholder="Platform On-Call" />
        <TextInput label="Description" value={form.description} onInput={setField('description')} placeholder="Optional description" />
        <SelectInput label="Repeat Count" value={form.repeat} onChange={setField('repeat')} options={REPEAT_OPTIONS} />
        <div class="form-actions">
          <button class="btn btn-secondary" onClick={() => setModalOpen(false)}>Cancel</button>
          <button class="btn btn-primary" onClick={handleSave} disabled={saving}>
            {saving ? 'Saving...' : editing ? 'Save Changes' : 'Create Policy'}
          </button>
        </div>
      </Modal>

      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDelete}
        title="Delete Escalation Policy"
        message={`Are you sure you want to delete "${deleteTarget?.name}"? Services using this policy will need to be updated.`}
      />
    </div>
  )
}
