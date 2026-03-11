import { useState } from 'preact/hooks'
import { useApi } from '../hooks.js'
import { useAuth } from '../auth.jsx'
import { apiPost, apiDelete } from '../api.js'
import { Modal } from '../components/modal.jsx'
import { SelectInput } from '../components/form-field.jsx'
import { ConfirmDialog } from '../components/confirm-dialog.jsx'
import { useToast } from '../components/toast.jsx'
import { StatusBadge } from '../components/status-badge.jsx'

const MEMBER_ROLES = [
  { value: 'member', label: 'Member' },
  { value: 'admin', label: 'Admin' },
]

export function TeamDetail({ id }) {
  const { data: team, loading } = useApi(`/teams/${id}`)
  const { data: members, refetch: refetchMembers } = useApi(`/teams/${id}/members`)
  const { data: users } = useApi('/users')
  const { user: currentUser } = useAuth()
  const isAdmin = currentUser?.role === 'admin'
  const toast = useToast()

  const [addModalOpen, setAddModalOpen] = useState(false)
  const [addForm, setAddForm] = useState({ user_id: '', role: 'member' })
  const [addErrors, setAddErrors] = useState({})
  const [removeTarget, setRemoveTarget] = useState(null)

  const memberUserIds = new Set((members || []).map(m => m.user_id))
  const availableUsers = (users || []).filter(u => !memberUserIds.has(u.id))
  const userOptions = availableUsers.map(u => ({ value: u.id, label: `${u.name} (${u.email})` }))

  const userName = (uid) => {
    const u = (users || []).find(u => u.id === uid)
    return u ? u.name : uid
  }

  const userEmail = (uid) => {
    const u = (users || []).find(u => u.id === uid)
    return u ? u.email : ''
  }

  const handleAddMember = async () => {
    if (!addForm.user_id) {
      setAddErrors({ user_id: 'Select a user' })
      return
    }
    const { error } = await apiPost(`/teams/${id}/members`, addForm)
    if (error) {
      toast.error(error)
      return
    }
    toast.success('Member added')
    setAddModalOpen(false)
    setAddForm({ user_id: '', role: 'member' })
    refetchMembers()
  }

  const handleRemoveMember = async () => {
    const { error } = await apiDelete(`/teams/${id}/members/${removeTarget.user_id}`)
    if (error) {
      toast.error(error)
    } else {
      toast.success('Member removed')
      refetchMembers()
    }
    setRemoveTarget(null)
  }

  if (loading) return <div class="loading">Loading...</div>
  if (!team) return <div class="page"><p>Team not found</p></div>

  return (
    <div class="page">
      <div class="page-header">
        <div>
          <a href="/teams" class="breadcrumb">Teams</a>
          <h1>{team.name}</h1>
        </div>
      </div>

      <div class="detail-grid">
        <div class="detail-card">
          <h3>Details</h3>
          {team.description && (
            <div class="detail-row">
              <span class="detail-label">Description</span>
              <span class="text-muted">{team.description}</span>
            </div>
          )}
        </div>

        <div class="detail-card">
          <div class="card-header-row">
            <h3>Members</h3>
            {isAdmin && userOptions.length > 0 && (
              <button class="btn btn-primary btn-sm" onClick={() => { setAddModalOpen(true); setAddForm({ user_id: '', role: 'member' }); setAddErrors({}) }}>
                Add Member
              </button>
            )}
          </div>
          {!members || members.length === 0 ? (
            <p class="text-muted">No members. Add users to this team.</p>
          ) : (
            <div class="sub-list">
              {members.map(m => (
                <div key={m.user_id} class="sub-list-item">
                  <div>
                    <span class="bold">{userName(m.user_id)}</span>
                    <span class="text-muted" style="margin-left: 8px">{userEmail(m.user_id)}</span>
                    <StatusBadge status={m.role} />
                  </div>
                  {isAdmin && (
                    <div class="sub-list-actions">
                      <button class="btn-icon btn-icon-danger" onClick={() => setRemoveTarget(m)} title="Remove">&times;</button>
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      <Modal open={addModalOpen} onClose={() => setAddModalOpen(false)} title="Add Team Member">
        <SelectInput
          label="User"
          value={addForm.user_id}
          onChange={(e) => setAddForm(prev => ({ ...prev, user_id: e.target.value }))}
          options={userOptions}
          placeholder="Select a user..."
          error={addErrors.user_id}
        />
        <SelectInput
          label="Role"
          value={addForm.role}
          onChange={(e) => setAddForm(prev => ({ ...prev, role: e.target.value }))}
          options={MEMBER_ROLES}
        />
        <div class="form-actions">
          <button class="btn btn-secondary" onClick={() => setAddModalOpen(false)}>Cancel</button>
          <button class="btn btn-primary" onClick={handleAddMember}>Add Member</button>
        </div>
      </Modal>

      <ConfirmDialog
        open={!!removeTarget}
        onClose={() => setRemoveTarget(null)}
        onConfirm={handleRemoveMember}
        title="Remove Member"
        message={`Remove ${userName(removeTarget?.user_id)} from ${team.name}?`}
      />
    </div>
  )
}
