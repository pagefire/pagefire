import { useState } from 'preact/hooks'
import { useApi } from '../hooks.js'
import { useAuth } from '../auth.jsx'
import { apiPost, apiPut, apiDelete } from '../api.js'
import { EmptyState } from '../components/empty-state.jsx'
import { TimeAgo } from '../components/time-ago.jsx'
import { Modal } from '../components/modal.jsx'
import { TextInput, SelectInput } from '../components/form-field.jsx'
import { ConfirmDialog } from '../components/confirm-dialog.jsx'
import { useToast } from '../components/toast.jsx'

const emptyForm = { name: '', description: '', escalation_policy_id: '' }

export function Services() {
  const { data: services, loading, refetch } = useApi('/services')
  const { data: policies } = useApi('/escalation-policies')
  const { user: currentUser } = useAuth()
  const isAdmin = currentUser?.role === 'admin'
  const toast = useToast()

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState(null)
  const [form, setForm] = useState(emptyForm)
  const [errors, setErrors] = useState({})
  const [saving, setSaving] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState(null)

  const policyOptions = (policies || []).map(p => ({ value: p.id, label: p.name }))

  const openCreate = () => {
    setEditing(null)
    setForm(emptyForm)
    setErrors({})
    setModalOpen(true)
  }

  const openEdit = (svc, e) => {
    e.stopPropagation()
    setEditing(svc)
    setForm({ name: svc.name, description: svc.description || '', escalation_policy_id: svc.escalation_policy_id || '' })
    setErrors({})
    setModalOpen(true)
  }

  const validate = () => {
    const errs = {}
    if (!form.name.trim()) errs.name = 'Name is required'
    if (!form.escalation_policy_id) errs.escalation_policy_id = 'Escalation policy is required'
    setErrors(errs)
    return Object.keys(errs).length === 0
  }

  const handleSave = async () => {
    if (!validate()) return
    setSaving(true)
    const payload = { name: form.name.trim(), description: form.description.trim(), escalation_policy_id: form.escalation_policy_id }
    const { error } = editing
      ? await apiPut(`/services/${editing.id}`, payload)
      : await apiPost('/services', payload)
    setSaving(false)
    if (error) {
      toast.error(error)
      return
    }
    toast.success(editing ? 'Service updated' : 'Service created')
    setModalOpen(false)
    refetch()
  }

  const handleDelete = async () => {
    const { error } = await apiDelete(`/services/${deleteTarget.id}`)
    if (error) {
      toast.error(error)
    } else {
      toast.success('Service deleted')
      refetch()
    }
    setDeleteTarget(null)
  }

  const setField = (field) => (e) => {
    setForm(prev => ({ ...prev, [field]: e.target.value }))
    if (errors[field]) setErrors(prev => ({ ...prev, [field]: null }))
  }

  const policyName = (id) => {
    const p = (policies || []).find(p => p.id === id)
    return p ? p.name : '—'
  }

  return (
    <div class="page">
      <div class="page-header">
        <h1>Services</h1>
        <div class="actions">
          {isAdmin && <button class="btn btn-primary" onClick={openCreate}>Add Service</button>}
        </div>
      </div>

      {loading ? (
        <div class="loading">Loading...</div>
      ) : !services || services.length === 0 ? (
        <EmptyState message="No services configured" />
      ) : (
        <table class="data-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Escalation Policy</th>
              <th>Created</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {services.map(s => (
              <tr key={s.id} class="clickable-row" onClick={() => { window.location.href = `/services/${s.id}` }}>
                <td class="bold">{s.name}</td>
                <td class="text-muted">{policyName(s.escalation_policy_id)}</td>
                <td><TimeAgo time={s.created_at} /></td>
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

      <Modal open={modalOpen} onClose={() => setModalOpen(false)} title={editing ? 'Edit Service' : 'Add Service'}>
        <TextInput label="Name" value={form.name} onInput={setField('name')} error={errors.name} placeholder="API Gateway" />
        <TextInput label="Description" value={form.description} onInput={setField('description')} placeholder="Optional description" />
        <SelectInput
          label="Escalation Policy"
          value={form.escalation_policy_id}
          onChange={setField('escalation_policy_id')}
          options={policyOptions}
          placeholder="Select a policy..."
          error={errors.escalation_policy_id}
        />
        <div class="form-actions">
          <button class="btn btn-secondary" onClick={() => setModalOpen(false)}>Cancel</button>
          <button class="btn btn-primary" onClick={handleSave} disabled={saving}>
            {saving ? 'Saving...' : editing ? 'Save Changes' : 'Create Service'}
          </button>
        </div>
      </Modal>

      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDelete}
        title="Delete Service"
        message={`Are you sure you want to delete "${deleteTarget?.name}"? All associated alerts, integration keys, and routing rules will be affected.`}
      />
    </div>
  )
}
