import { useState } from 'preact/hooks'
import { useApi } from '../hooks.js'
import { useAuth } from '../auth.jsx'
import { apiPost, apiPut, apiDelete } from '../api.js'
import { Modal } from '../components/modal.jsx'
import { TextInput, SelectInput } from '../components/form-field.jsx'
import { ConfirmDialog } from '../components/confirm-dialog.jsx'
import { useToast } from '../components/toast.jsx'
import { TimeAgo } from '../components/time-ago.jsx'

const CONTACT_TYPES = [
  { value: 'email', label: 'Email' },
  { value: 'sms', label: 'SMS' },
  { value: 'phone', label: 'Phone Call' },
  { value: 'slack_dm', label: 'Slack DM' },
  { value: 'webhook', label: 'Webhook' },
]

const CONTACT_PLACEHOLDERS = {
  email: 'you@company.com',
  sms: '+12025551234',
  phone: '+12025551234',
  slack_dm: 'U01ABC123',
  webhook: 'https://hooks.example.com/notify',
}

export function Profile() {
  const { user, checkAuth } = useAuth()
  const toast = useToast()

  if (!user) return <div class="loading">Loading...</div>

  return (
    <div class="page">
      <div class="page-header">
        <h1>Profile &amp; Settings</h1>
      </div>

      <div class="detail-grid">
        <ProfileCard user={user} toast={toast} checkAuth={checkAuth} />
        <PasswordCard toast={toast} />
      </div>

      <ContactMethodsSection userId={user.id} toast={toast} />
      <NotificationRulesSection userId={user.id} toast={toast} />
      <APITokensSection toast={toast} />
    </div>
  )
}

// --- Profile Info ---
function ProfileCard({ user }) {
  return (
    <div class="detail-card">
      <h3>Account</h3>
      <div class="detail-row">
        <span class="detail-label">Name</span>
        <span>{user.name}</span>
      </div>
      <div class="detail-row">
        <span class="detail-label">Email</span>
        <span>{user.email}</span>
      </div>
      <div class="detail-row">
        <span class="detail-label">Role</span>
        <span class="source-tag">{user.role}</span>
      </div>
      <div class="detail-row">
        <span class="detail-label">Timezone</span>
        <span>{user.timezone || 'UTC'}</span>
      </div>
    </div>
  )
}

// --- Password Change ---
function PasswordCard({ toast }) {
  const [form, setForm] = useState({ current_password: '', new_password: '', confirm: '' })
  const [errors, setErrors] = useState({})
  const [saving, setSaving] = useState(false)

  const handleChange = async () => {
    const errs = {}
    if (!form.current_password) errs.current_password = 'Required'
    if (!form.new_password) errs.new_password = 'Required'
    else if (form.new_password.length < 8 || !/[A-Z]/.test(form.new_password) || !/[a-z]/.test(form.new_password) || !/[0-9]/.test(form.new_password)) errs.new_password = 'Min 8 chars, upper + lower + digit'
    if (form.new_password !== form.confirm) errs.confirm = 'Passwords do not match'
    setErrors(errs)
    if (Object.keys(errs).length > 0) return

    setSaving(true)
    const { error } = await apiPut('/auth/password', {
      current_password: form.current_password,
      new_password: form.new_password,
    })
    setSaving(false)
    if (error) {
      toast.error(error)
      return
    }
    toast.success('Password changed')
    setForm({ current_password: '', new_password: '', confirm: '' })
  }

  return (
    <div class="detail-card">
      <h3>Change Password</h3>
      <TextInput
        label="Current Password"
        type="password"
        value={form.current_password}
        onInput={(e) => setForm(prev => ({ ...prev, current_password: e.target.value }))}
        error={errors.current_password}
      />
      <TextInput
        label="New Password"
        type="password"
        value={form.new_password}
        onInput={(e) => setForm(prev => ({ ...prev, new_password: e.target.value }))}
        error={errors.new_password}
        placeholder="Min 8 chars, upper + lower + digit"
      />
      <TextInput
        label="Confirm New Password"
        type="password"
        value={form.confirm}
        onInput={(e) => setForm(prev => ({ ...prev, confirm: e.target.value }))}
        error={errors.confirm}
      />
      <div style="margin-top: 12px">
        <button class="btn btn-primary" onClick={handleChange} disabled={saving}>
          {saving ? 'Saving...' : 'Change Password'}
        </button>
      </div>
    </div>
  )
}

// --- Contact Methods ---
function ContactMethodsSection({ userId, toast }) {
  const { data: methods, refetch } = useApi(`/users/${userId}/contact-methods`)

  const [modalOpen, setModalOpen] = useState(false)
  const [form, setForm] = useState({ type: 'email', value: '' })
  const [errors, setErrors] = useState({})
  const [deleteTarget, setDeleteTarget] = useState(null)

  const handleCreate = async () => {
    const errs = {}
    if (!form.value.trim()) errs.value = 'Value is required'
    setErrors(errs)
    if (Object.keys(errs).length > 0) return

    const { error } = await apiPost(`/users/${userId}/contact-methods`, { type: form.type, value: form.value.trim() })
    if (error) {
      toast.error(error)
      return
    }
    toast.success('Contact method added')
    setModalOpen(false)
    setForm({ type: 'email', value: '' })
    refetch()
  }

  const handleDelete = async () => {
    const { error } = await apiDelete(`/users/${userId}/contact-methods/${deleteTarget.id}`)
    if (error) {
      toast.error(error)
    } else {
      toast.success('Contact method removed')
      refetch()
    }
    setDeleteTarget(null)
  }

  return (
    <div class="detail-card" style="margin-top: 20px">
      <div class="card-header-row">
        <h3>Contact Methods</h3>
        <button class="btn btn-primary btn-sm" onClick={() => { setModalOpen(true); setForm({ type: 'email', value: '' }); setErrors({}) }}>
          Add Contact Method
        </button>
      </div>
      <p class="text-muted" style="margin-bottom: 12px; font-size: 13px">
        How you receive alert notifications. Add at least one to get paged.
      </p>

      {!methods || methods.length === 0 ? (
        <p class="text-muted">No contact methods configured. You won't receive any notifications.</p>
      ) : (
        <div class="sub-list">
          {methods.map(m => (
            <div key={m.id} class="sub-list-item">
              <div>
                <span class="source-tag">{m.type}</span>
                <span class="mono" style="margin-left: 8px">{m.value}</span>
              </div>
              <div class="sub-list-actions">
                <button class="btn-icon btn-icon-danger" onClick={() => setDeleteTarget(m)} title="Delete">&times;</button>
              </div>
            </div>
          ))}
        </div>
      )}

      <Modal open={modalOpen} onClose={() => setModalOpen(false)} title="Add Contact Method">
        <SelectInput
          label="Type"
          value={form.type}
          onChange={(e) => setForm(prev => ({ ...prev, type: e.target.value, value: '' }))}
          options={CONTACT_TYPES}
        />
        <TextInput
          label="Value"
          value={form.value}
          onInput={(e) => setForm(prev => ({ ...prev, value: e.target.value }))}
          error={errors.value}
          placeholder={CONTACT_PLACEHOLDERS[form.type] || ''}
        />
        <div class="form-actions">
          <button class="btn btn-secondary" onClick={() => setModalOpen(false)}>Cancel</button>
          <button class="btn btn-primary" onClick={handleCreate}>Add</button>
        </div>
      </Modal>

      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDelete}
        title="Remove Contact Method"
        message={`Remove ${deleteTarget?.type} contact "${deleteTarget?.value}"? You'll stop receiving notifications through this method.`}
      />
    </div>
  )
}

// --- Notification Rules ---
function NotificationRulesSection({ userId, toast }) {
  const { data: rules, refetch } = useApi(`/users/${userId}/notification-rules`)
  const { data: methods } = useApi(`/users/${userId}/contact-methods`)

  const [modalOpen, setModalOpen] = useState(false)
  const [form, setForm] = useState({ contact_method_id: '', delay_minutes: '0' })
  const [errors, setErrors] = useState({})
  const [deleteTarget, setDeleteTarget] = useState(null)

  const methodOptions = (methods || []).map(m => ({ value: m.id, label: `${m.type}: ${m.value}` }))

  const methodLabel = (cmId) => {
    const m = (methods || []).find(m => m.id === cmId)
    return m ? `${m.type}: ${m.value}` : cmId
  }

  const handleCreate = async () => {
    const errs = {}
    if (!form.contact_method_id) errs.contact_method_id = 'Select a contact method'
    const delay = parseInt(form.delay_minutes, 10)
    if (isNaN(delay) || delay < 0) errs.delay_minutes = 'Must be 0 or more'
    setErrors(errs)
    if (Object.keys(errs).length > 0) return

    const { error } = await apiPost(`/users/${userId}/notification-rules`, {
      contact_method_id: form.contact_method_id,
      delay_minutes: delay,
    })
    if (error) {
      toast.error(error)
      return
    }
    toast.success('Notification rule added')
    setModalOpen(false)
    setForm({ contact_method_id: '', delay_minutes: '0' })
    refetch()
  }

  const handleDelete = async () => {
    const { error } = await apiDelete(`/users/${userId}/notification-rules/${deleteTarget.id}`)
    if (error) {
      toast.error(error)
    } else {
      toast.success('Notification rule removed')
      refetch()
    }
    setDeleteTarget(null)
  }

  return (
    <div class="detail-card" style="margin-top: 20px">
      <div class="card-header-row">
        <h3>Notification Rules</h3>
        <button class="btn btn-primary btn-sm" onClick={() => { setModalOpen(true); setForm({ contact_method_id: '', delay_minutes: '0' }); setErrors({}) }}>
          Add Rule
        </button>
      </div>
      <p class="text-muted" style="margin-bottom: 12px; font-size: 13px">
        When you're paged, each rule fires after its delay. Add multiple rules to get notified on different channels.
      </p>

      {!rules || rules.length === 0 ? (
        <p class="text-muted">No notification rules. Add a rule to control how you get paged.</p>
      ) : (
        <table class="data-table">
          <thead>
            <tr>
              <th>Contact Method</th>
              <th>Delay</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {rules.map(r => (
              <tr key={r.id}>
                <td>{methodLabel(r.contact_method_id)}</td>
                <td>{r.delay_minutes === 0 ? 'Immediately' : `After ${r.delay_minutes} min`}</td>
                <td class="row-actions">
                  <button class="btn-icon btn-icon-danger" onClick={() => setDeleteTarget(r)} title="Delete">&times;</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <Modal open={modalOpen} onClose={() => setModalOpen(false)} title="Add Notification Rule">
        {methodOptions.length === 0 ? (
          <p class="text-muted">Add a contact method first before creating notification rules.</p>
        ) : (
          <>
            <SelectInput
              label="Contact Method"
              value={form.contact_method_id}
              onChange={(e) => setForm(prev => ({ ...prev, contact_method_id: e.target.value }))}
              options={methodOptions}
              placeholder="Select contact method..."
              error={errors.contact_method_id}
            />
            <TextInput
              label="Delay (minutes)"
              type="number"
              value={form.delay_minutes}
              onInput={(e) => setForm(prev => ({ ...prev, delay_minutes: e.target.value }))}
              error={errors.delay_minutes}
              placeholder="0 = immediately"
            />
            <div class="form-actions">
              <button class="btn btn-secondary" onClick={() => setModalOpen(false)}>Cancel</button>
              <button class="btn btn-primary" onClick={handleCreate}>Add Rule</button>
            </div>
          </>
        )}
      </Modal>

      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDelete}
        title="Remove Notification Rule"
        message="Remove this notification rule? You may miss pages if you don't have other rules."
      />
    </div>
  )
}

// --- API Tokens ---
function APITokensSection({ toast }) {
  const { data: tokens, refetch } = useApi('/auth/tokens')

  const [modalOpen, setModalOpen] = useState(false)
  const [name, setName] = useState('')
  const [nameError, setNameError] = useState(null)
  const [newToken, setNewToken] = useState(null)
  const [deleteTarget, setDeleteTarget] = useState(null)

  const handleCreate = async () => {
    if (!name.trim()) {
      setNameError('Name is required')
      return
    }
    const { data, error } = await apiPost('/auth/tokens', { name: name.trim() })
    if (error) {
      toast.error(error)
      return
    }
    setNewToken(data.token)
    toast.success('Token created')
    refetch()
  }

  const handleDelete = async () => {
    const { error } = await apiDelete(`/auth/tokens/${deleteTarget.id}`)
    if (error) {
      toast.error(error)
    } else {
      toast.success('Token revoked')
      refetch()
    }
    setDeleteTarget(null)
  }

  return (
    <div class="detail-card" style="margin-top: 20px">
      <div class="card-header-row">
        <h3>API Tokens</h3>
        <button class="btn btn-primary btn-sm" onClick={() => { setModalOpen(true); setName(''); setNameError(null); setNewToken(null) }}>
          Create Token
        </button>
      </div>
      <p class="text-muted" style="margin-bottom: 12px; font-size: 13px">
        Personal API tokens for programmatic access. Use as Bearer token in the Authorization header.
      </p>

      {!tokens || tokens.length === 0 ? (
        <p class="text-muted">No API tokens.</p>
      ) : (
        <table class="data-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Preview</th>
              <th>Created</th>
              <th>Last Used</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {tokens.map(t => (
              <tr key={t.id}>
                <td class="bold">{t.name}</td>
                <td class="mono text-muted">{t.prefix}...</td>
                <td><TimeAgo time={t.created_at} /></td>
                <td>{t.last_used ? <TimeAgo time={t.last_used} /> : <span class="text-muted">Never</span>}</td>
                <td class="row-actions">
                  <button class="btn-icon btn-icon-danger" onClick={() => setDeleteTarget(t)} title="Revoke">&times;</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <Modal open={modalOpen} onClose={() => setModalOpen(false)} title={newToken ? 'Token Created' : 'Create API Token'}>
        {newToken ? (
          <div>
            <p class="text-muted" style="margin-bottom: 12px">Copy this token now — it won't be shown again.</p>
            <div class="secret-display">
              <code class="mono">{newToken}</code>
            </div>
            <div class="form-actions">
              <button class="btn btn-primary" onClick={() => setModalOpen(false)}>Done</button>
            </div>
          </div>
        ) : (
          <div>
            <TextInput
              label="Token Name"
              value={name}
              onInput={(e) => { setName(e.target.value); setNameError(null) }}
              error={nameError}
              placeholder="e.g. CI/CD Pipeline"
            />
            <div class="form-actions">
              <button class="btn btn-secondary" onClick={() => setModalOpen(false)}>Cancel</button>
              <button class="btn btn-primary" onClick={handleCreate}>Create Token</button>
            </div>
          </div>
        )}
      </Modal>

      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDelete}
        title="Revoke API Token"
        message={`Revoke "${deleteTarget?.name}"? Any scripts using this token will stop working.`}
      />
    </div>
  )
}
