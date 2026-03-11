import { useState } from 'preact/hooks'
import { useApi } from '../hooks.js'
import { useAuth } from '../auth.jsx'
import { apiGet, apiPost, apiDelete } from '../api.js'
import { Modal } from '../components/modal.jsx'
import { SelectInput, TextInput } from '../components/form-field.jsx'
import { ConfirmDialog } from '../components/confirm-dialog.jsx'
import { useToast } from '../components/toast.jsx'
import { TimeAgo } from '../components/time-ago.jsx'

const ROTATION_TYPES = [
  { value: 'daily', label: 'Daily' },
  { value: 'weekly', label: 'Weekly' },
  { value: 'custom', label: 'Custom' },
]

export function ScheduleDetail({ id }) {
  const { data: schedule, loading } = useApi(`/schedules/${id}`)
  const { data: oncall } = useApi(`/oncall/${id}`)
  const { data: rotations, refetch: refetchRotations } = useApi(`/schedules/${id}/rotations`)
  const { data: overrides, refetch: refetchOverrides } = useApi(`/schedule-overrides/${id}/overrides`)
  const { data: users } = useApi('/users')
  const { user: currentUser } = useAuth()
  const isAdmin = currentUser?.role === 'admin'
  const toast = useToast()

  // Override state
  const [overrideModalOpen, setOverrideModalOpen] = useState(false)
  const [overrideForm, setOverrideForm] = useState({ replace_user: '', override_user: '', start_time: '', end_time: '' })
  const [overrideErrors, setOverrideErrors] = useState({})
  const [deleteOverride, setDeleteOverride] = useState(null)

  // Rotation state
  const [rotModalOpen, setRotModalOpen] = useState(false)
  const [rotForm, setRotForm] = useState({ name: '', type: 'weekly', shift_length: '1', handoff_time: '09:00' })
  const [rotErrors, setRotErrors] = useState({})
  const [deleteRotation, setDeleteRotation] = useState(null)

  // Participant state
  const [partModalOpen, setPartModalOpen] = useState(false)
  const [partRotationId, setPartRotationId] = useState(null)
  const [partForm, setPartForm] = useState({ user_id: '' })
  const [partErrors, setPartErrors] = useState({})
  const [deletePart, setDeletePart] = useState(null)

  // Participants per rotation (loaded on expand)
  const [rotationParticipants, setRotationParticipants] = useState({})
  const [expandedRotation, setExpandedRotation] = useState(null)

  const userOptions = (users || []).map(u => ({ value: u.id, label: u.name }))

  const userName = (uid) => {
    const u = (users || []).find(u => u.id === uid)
    return u ? u.name : uid
  }

  // --- Participants ---
  const loadParticipants = async (rotId) => {
    const { data, error } = await apiGet(`/schedules/${id}/rotations/${rotId}/participants`)
    if (!error) {
      setRotationParticipants(prev => ({ ...prev, [rotId]: data || [] }))
    }
  }

  const toggleRotation = (rotId) => {
    if (expandedRotation === rotId) {
      setExpandedRotation(null)
    } else {
      setExpandedRotation(rotId)
      if (!rotationParticipants[rotId]) {
        loadParticipants(rotId)
      }
    }
  }

  const openAddParticipant = (rotId) => {
    setPartRotationId(rotId)
    setPartForm({ user_id: '' })
    setPartErrors({})
    setPartModalOpen(true)
  }

  const handleAddParticipant = async () => {
    if (!partForm.user_id) {
      setPartErrors({ user_id: 'Select a user' })
      return
    }
    const { error } = await apiPost(`/schedules/${id}/rotations/${partRotationId}/participants`, { user_id: partForm.user_id })
    if (error) {
      toast.error(error)
      return
    }
    toast.success('Participant added')
    setPartModalOpen(false)
    loadParticipants(partRotationId)
  }

  const handleDeleteParticipant = async () => {
    const { error } = await apiDelete(`/schedules/${id}/rotations/${deletePart.rotation_id}/participants/${deletePart.id}`)
    if (error) {
      toast.error(error)
    } else {
      toast.success('Participant removed')
      loadParticipants(deletePart.rotation_id)
    }
    setDeletePart(null)
  }

  // --- Rotations ---
  const openCreateRotation = () => {
    setRotForm({ name: '', type: 'weekly', shift_length: '1', handoff_time: '09:00' })
    setRotErrors({})
    setRotModalOpen(true)
  }

  const handleCreateRotation = async () => {
    if (!rotForm.name.trim()) {
      setRotErrors({ name: 'Name is required' })
      return
    }
    const payload = {
      name: rotForm.name.trim(),
      type: rotForm.type,
      shift_length: parseInt(rotForm.shift_length, 10) || 1,
      handoff_time: rotForm.handoff_time || '09:00',
      start_time: new Date().toISOString(),
    }
    const { error } = await apiPost(`/schedules/${id}/rotations`, payload)
    if (error) {
      toast.error(error)
      return
    }
    toast.success('Rotation created')
    setRotModalOpen(false)
    refetchRotations()
  }

  const handleDeleteRotation = async () => {
    const { error } = await apiDelete(`/schedules/${id}/rotations/${deleteRotation.id}`)
    if (error) {
      toast.error(error)
    } else {
      toast.success('Rotation deleted')
      refetchRotations()
    }
    setDeleteRotation(null)
  }

  // --- Overrides ---
  const openCreateOverride = () => {
    const now = new Date()
    const tomorrow = new Date(now)
    tomorrow.setDate(tomorrow.getDate() + 1)
    setOverrideForm({
      replace_user: '',
      override_user: '',
      start_time: formatDateTimeLocal(now),
      end_time: formatDateTimeLocal(tomorrow),
    })
    setOverrideErrors({})
    setOverrideModalOpen(true)
  }

  const handleCreateOverride = async () => {
    const errs = {}
    if (!overrideForm.replace_user) errs.replace_user = 'Select who to replace'
    if (!overrideForm.override_user) errs.override_user = 'Select who takes over'
    if (!overrideForm.start_time) errs.start_time = 'Start time is required'
    if (!overrideForm.end_time) errs.end_time = 'End time is required'
    if (overrideForm.start_time && overrideForm.end_time && overrideForm.end_time <= overrideForm.start_time) {
      errs.end_time = 'End time must be after start time'
    }
    if (overrideForm.replace_user && overrideForm.override_user && overrideForm.replace_user === overrideForm.override_user) {
      errs.override_user = 'Cannot swap with the same person'
    }
    setOverrideErrors(errs)
    if (Object.keys(errs).length > 0) return

    const payload = {
      replace_user: overrideForm.replace_user,
      override_user: overrideForm.override_user,
      start_time: new Date(overrideForm.start_time).toISOString(),
      end_time: new Date(overrideForm.end_time).toISOString(),
    }
    const { error } = await apiPost(`/schedule-overrides/${id}/overrides`, payload)
    if (error) {
      toast.error(error)
      return
    }
    toast.success('Override created')
    setOverrideModalOpen(false)
    refetchOverrides()
  }

  const handleDeleteOverride = async () => {
    const { error } = await apiDelete(`/schedule-overrides/${id}/overrides/${deleteOverride.id}`)
    if (error) {
      toast.error(error)
    } else {
      toast.success('Override deleted')
      refetchOverrides()
    }
    setDeleteOverride(null)
  }

  if (loading) return <div class="loading">Loading...</div>
  if (!schedule) return <div class="page"><p>Schedule not found</p></div>

  const now = new Date()

  return (
    <div class="page">
      <div class="page-header">
        <div>
          <a href="/schedules" class="breadcrumb">Schedules</a>
          <h1>{schedule.name}</h1>
        </div>
      </div>

      <div class="detail-grid">
        <div class="detail-card">
          <h3>On-Call Now</h3>
          {oncall && oncall.user_id ? (
            <div class="oncall-badge">
              <span class="oncall-dot" />
              <span>{oncall.user_name || oncall.user_id}</span>
            </div>
          ) : (
            <p class="text-muted">No one on call</p>
          )}
        </div>

        <div class="detail-card">
          <h3>Details</h3>
          <div class="detail-row">
            <span class="detail-label">Timezone</span>
            <span>{schedule.timezone}</span>
          </div>
          {schedule.description && (
            <div class="detail-row">
              <span class="detail-label">Description</span>
              <span>{schedule.description}</span>
            </div>
          )}
        </div>
      </div>

      {/* Rotations — admin can add/delete, all can view */}
      <div class="detail-card" style="margin-top: 20px">
        <div class="card-header-row">
          <h3>Rotations</h3>
          {isAdmin && (
            <button class="btn btn-primary btn-sm" onClick={openCreateRotation}>
              Add Rotation
            </button>
          )}
        </div>
        {!rotations || rotations.length === 0 ? (
          <p class="text-muted">No rotations. Add one to start scheduling on-call shifts.</p>
        ) : (
          <div class="sub-list">
            {rotations.map(r => (
              <div key={r.id}>
                <div class="sub-list-item" style="cursor: pointer" onClick={() => toggleRotation(r.id)}>
                  <div>
                    <span class="bold">{r.name}</span>
                    <span class="text-muted" style="margin-left: 8px">
                      {r.type} &middot; {r.shift_length} shift{r.shift_length > 1 ? 's' : ''} &middot; handoff {r.handoff_time || '09:00'}
                    </span>
                    <span class="text-muted" style="margin-left: 8px; font-size: 11px">
                      {expandedRotation === r.id ? '▼' : '▶'} participants
                    </span>
                  </div>
                  {isAdmin && (
                    <div class="sub-list-actions">
                      <button class="btn-icon btn-icon-danger" onClick={(e) => { e.stopPropagation(); setDeleteRotation(r) }} title="Delete rotation">&times;</button>
                    </div>
                  )}
                </div>
                {expandedRotation === r.id && (
                  <div style="padding: 8px 0 8px 24px; border-bottom: 1px solid var(--color-border)">
                    {isAdmin && (
                      <button class="btn btn-secondary btn-sm" style="margin-bottom: 8px" onClick={() => openAddParticipant(r.id)}>
                        Add Participant
                      </button>
                    )}
                    {(rotationParticipants[r.id] || []).length === 0 ? (
                      <p class="text-muted" style="font-size: 13px">No participants in this rotation.</p>
                    ) : (
                      <div class="sub-list">
                        {(rotationParticipants[r.id] || []).map((p, idx) => (
                          <div key={p.id} class="sub-list-item">
                            <div>
                              <span class="text-muted" style="margin-right: 8px; font-size: 12px">#{idx + 1}</span>
                              <span class="bold">{userName(p.user_id)}</span>
                            </div>
                            {isAdmin && (
                              <div class="sub-list-actions">
                                <button class="btn-icon btn-icon-danger" onClick={() => setDeletePart({ ...p, rotation_id: r.id })} title="Remove">&times;</button>
                              </div>
                            )}
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Schedule Overrides — available to all users */}
      <div class="detail-card" style="margin-top: 20px">
        <div class="card-header-row">
          <h3>Overrides</h3>
          <button class="btn btn-primary btn-sm" onClick={openCreateOverride}>
            Create Override
          </button>
        </div>
        <p class="text-muted" style="margin-bottom: 12px; font-size: 13px">
          Temporarily swap on-call duty with another person. Active overrides replace the scheduled person.
        </p>
        {!overrides || overrides.length === 0 ? (
          <p class="text-muted">No overrides scheduled.</p>
        ) : (
          <table class="data-table">
            <thead>
              <tr>
                <th>Replacing</th>
                <th>With</th>
                <th>Start</th>
                <th>End</th>
                <th>Status</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {overrides.map(o => {
                const start = new Date(o.start_time)
                const end = new Date(o.end_time)
                const isActive = now >= start && now < end
                const isExpired = now >= end
                return (
                  <tr key={o.id}>
                    <td class="bold">{userName(o.replace_user)}</td>
                    <td class="bold">{userName(o.override_user)}</td>
                    <td><TimeAgo time={o.start_time} /></td>
                    <td><TimeAgo time={o.end_time} /></td>
                    <td>
                      <span class={`source-tag ${isActive ? 'tag-active' : isExpired ? 'tag-expired' : ''}`}>
                        {isActive ? 'Active' : isExpired ? 'Expired' : 'Upcoming'}
                      </span>
                    </td>
                    <td class="row-actions">
                      {!isExpired && (
                        <button class="btn-icon btn-icon-danger" onClick={() => setDeleteOverride(o)} title="Delete">&times;</button>
                      )}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        )}
      </div>

      {/* Create Rotation Modal */}
      <Modal open={rotModalOpen} onClose={() => setRotModalOpen(false)} title="Add Rotation">
        <TextInput label="Name" value={rotForm.name} onInput={(e) => setRotForm(prev => ({ ...prev, name: e.target.value }))} error={rotErrors.name} placeholder="Primary" />
        <SelectInput
          label="Type"
          value={rotForm.type}
          onChange={(e) => setRotForm(prev => ({ ...prev, type: e.target.value }))}
          options={ROTATION_TYPES}
        />
        <TextInput label="Shift Length" value={rotForm.shift_length} onInput={(e) => setRotForm(prev => ({ ...prev, shift_length: e.target.value }))} type="number" placeholder="1" />
        <TextInput label="Handoff Time" value={rotForm.handoff_time} onInput={(e) => setRotForm(prev => ({ ...prev, handoff_time: e.target.value }))} type="time" />
        <div class="form-actions">
          <button class="btn btn-secondary" onClick={() => setRotModalOpen(false)}>Cancel</button>
          <button class="btn btn-primary" onClick={handleCreateRotation}>Create Rotation</button>
        </div>
      </Modal>

      {/* Add Participant Modal */}
      <Modal open={partModalOpen} onClose={() => setPartModalOpen(false)} title="Add Participant">
        <SelectInput
          label="User"
          value={partForm.user_id}
          onChange={(e) => setPartForm(prev => ({ ...prev, user_id: e.target.value }))}
          options={userOptions}
          placeholder="Select a user..."
          error={partErrors.user_id}
        />
        <div class="form-actions">
          <button class="btn btn-secondary" onClick={() => setPartModalOpen(false)}>Cancel</button>
          <button class="btn btn-primary" onClick={handleAddParticipant}>Add</button>
        </div>
      </Modal>

      {/* Create Override Modal */}
      <Modal open={overrideModalOpen} onClose={() => setOverrideModalOpen(false)} title="Create Schedule Override">
        <SelectInput
          label="Who to replace"
          value={overrideForm.replace_user}
          onChange={(e) => setOverrideForm(prev => ({ ...prev, replace_user: e.target.value }))}
          options={userOptions}
          placeholder="Select person going off-call..."
          error={overrideErrors.replace_user}
        />
        <SelectInput
          label="Who takes over"
          value={overrideForm.override_user}
          onChange={(e) => setOverrideForm(prev => ({ ...prev, override_user: e.target.value }))}
          options={userOptions}
          placeholder="Select person going on-call..."
          error={overrideErrors.override_user}
        />
        <TextInput
          label="Start"
          value={overrideForm.start_time}
          onInput={(e) => setOverrideForm(prev => ({ ...prev, start_time: e.target.value }))}
          error={overrideErrors.start_time}
          type="datetime-local"
        />
        <TextInput
          label="End"
          value={overrideForm.end_time}
          onInput={(e) => setOverrideForm(prev => ({ ...prev, end_time: e.target.value }))}
          error={overrideErrors.end_time}
          type="datetime-local"
        />
        <div class="form-actions">
          <button class="btn btn-secondary" onClick={() => setOverrideModalOpen(false)}>Cancel</button>
          <button class="btn btn-primary" onClick={handleCreateOverride}>Create Override</button>
        </div>
      </Modal>

      {/* Confirm Dialogs */}
      <ConfirmDialog
        open={!!deleteRotation}
        onClose={() => setDeleteRotation(null)}
        onConfirm={handleDeleteRotation}
        title="Delete Rotation"
        message={`Delete rotation "${deleteRotation?.name}"? All participants will be removed.`}
      />

      <ConfirmDialog
        open={!!deletePart}
        onClose={() => setDeletePart(null)}
        onConfirm={handleDeleteParticipant}
        title="Remove Participant"
        message={`Remove ${userName(deletePart?.user_id)} from this rotation?`}
      />

      <ConfirmDialog
        open={!!deleteOverride}
        onClose={() => setDeleteOverride(null)}
        onConfirm={handleDeleteOverride}
        title="Delete Override"
        message="Are you sure you want to delete this schedule override? The original rotation will resume."
      />
    </div>
  )
}

function formatDateTimeLocal(date) {
  const pad = (n) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}
