import { useState } from 'preact/hooks'
import { useApi } from '../hooks.js'
import { useAuth } from '../auth.jsx'
import { apiPost, apiPut, apiDelete } from '../api.js'
import { EmptyState } from '../components/empty-state.jsx'
import { StatusBadge } from '../components/status-badge.jsx'
import { Modal } from '../components/modal.jsx'
import { TextInput, SelectInput } from '../components/form-field.jsx'
import { ConfirmDialog } from '../components/confirm-dialog.jsx'
import { useToast } from '../components/toast.jsx'

const TIMEZONES = [
  'UTC', 'America/New_York', 'America/Chicago', 'America/Denver',
  'America/Los_Angeles', 'America/Sao_Paulo', 'Europe/London',
  'Europe/Berlin', 'Europe/Moscow', 'Asia/Kolkata', 'Asia/Shanghai',
  'Asia/Tokyo', 'Australia/Sydney', 'Pacific/Auckland',
].map(tz => ({ value: tz, label: tz }))

const emptyForm = { name: '', email: '', timezone: 'UTC', role: 'user' }

const ROLES = [
  { value: 'user', label: 'User' },
  { value: 'admin', label: 'Admin' },
]

export function Users() {
  const { data: users, loading, refetch } = useApi('/users')
  const { user: currentUser } = useAuth()
  const toast = useToast()

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState(null)
  const [form, setForm] = useState(emptyForm)
  const [errors, setErrors] = useState({})
  const [saving, setSaving] = useState(false)

  const [deleteTarget, setDeleteTarget] = useState(null)
  const [inviteUrl, setInviteUrl] = useState(null)

  const openCreate = () => {
    setEditing(null)
    setForm(emptyForm)
    setErrors({})
    setInviteUrl(null)
    setModalOpen(true)
  }

  const openEdit = (user) => {
    setEditing(user)
    setForm({ name: user.name, email: user.email, timezone: user.timezone || 'UTC' })
    setErrors({})
    setModalOpen(true)
  }

  const isAdmin = currentUser?.role === 'admin'

  const validate = () => {
    const errs = {}
    if (!form.name.trim()) errs.name = 'Name is required'
    if (!form.email.trim()) errs.email = 'Email is required'
    else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.email)) errs.email = 'Invalid email address'
    setErrors(errs)
    return Object.keys(errs).length === 0
  }

  const handleSave = async () => {
    if (!validate()) return
    setSaving(true)
    const payload = { name: form.name.trim(), email: form.email.trim(), timezone: form.timezone }
    if (!editing) {
      payload.role = form.role
    }
    const { data, error } = editing
      ? await apiPut(`/users/${editing.id}`, payload)
      : await apiPost('/users', payload)
    setSaving(false)
    if (error) {
      toast.error(error)
      return
    }
    if (!editing && data?.invite_url) {
      toast.success('User created — share the invite link')
      setInviteUrl(data.invite_url)
      refetch()
    } else {
      toast.success(editing ? 'User updated' : 'User created')
      setModalOpen(false)
      refetch()
    }
  }

  const handleDelete = async () => {
    const { error } = await apiDelete(`/users/${deleteTarget.id}`)
    if (error) {
      toast.error(error)
    } else {
      toast.success('User deleted')
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
        <h1>Users</h1>
        {isAdmin && (
          <div class="actions">
            <button class="btn btn-primary" onClick={openCreate}>Add User</button>
          </div>
        )}
      </div>

      {loading ? (
        <div class="loading">Loading...</div>
      ) : !users || users.length === 0 ? (
        <EmptyState message="No users" />
      ) : (
        <table class="data-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Email</th>
              <th>Role</th>
              <th>Timezone</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {users.map(u => (
              <tr key={u.id}>
                <td class="bold">{u.name}</td>
                <td>{u.email}</td>
                <td><StatusBadge status={u.role} /></td>
                <td class="text-muted">{u.timezone || '—'}</td>
                {isAdmin && (
                  <td class="row-actions">
                    <button class="btn-icon" onClick={() => openEdit(u)} title="Edit">&#9998;</button>
                    {currentUser?.id !== u.id && (
                      <button class="btn-icon btn-icon-danger" onClick={() => setDeleteTarget(u)} title="Delete">&times;</button>
                    )}
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <Modal open={modalOpen} onClose={() => { setModalOpen(false); setInviteUrl(null) }} title={inviteUrl ? 'Invite Link' : editing ? 'Edit User' : 'Add User'}>
        {inviteUrl ? (
          <div>
            <p class="text-muted" style="margin-bottom: 12px">Share this link with the user. They'll set their own password. The link expires in 7 days.</p>
            <div class="secret-display">
              <code class="mono" style="word-break: break-all">{inviteUrl}</code>
            </div>
            <div class="form-actions">
              <button class="btn btn-primary" onClick={() => { setModalOpen(false); setInviteUrl(null) }}>Done</button>
            </div>
          </div>
        ) : (
          <div>
            <TextInput label="Name" value={form.name} onInput={setField('name')} error={errors.name} placeholder="Jane Doe" />
            <TextInput label="Email" value={form.email} onInput={setField('email')} error={errors.email} placeholder="jane@example.com" type="email" />
            {!editing && (
              <SelectInput label="Role" value={form.role} onChange={setField('role')} options={ROLES} />
            )}
            <SelectInput label="Timezone" value={form.timezone} onChange={setField('timezone')} options={TIMEZONES} />
            {!editing && (
              <p class="text-muted" style="font-size: 12px; margin-top: 8px">An invite link will be generated. The user will set their own password.</p>
            )}
            <div class="form-actions">
              <button class="btn btn-secondary" onClick={() => setModalOpen(false)}>Cancel</button>
              <button class="btn btn-primary" onClick={handleSave} disabled={saving}>
                {saving ? 'Saving...' : editing ? 'Save Changes' : 'Create & Get Invite Link'}
              </button>
            </div>
          </div>
        )}
      </Modal>

      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDelete}
        title="Delete User"
        message={`Are you sure you want to delete "${deleteTarget?.name}"? This action cannot be undone.`}
      />
    </div>
  )
}
