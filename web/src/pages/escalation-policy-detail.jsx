import { useState } from 'preact/hooks'
import { useApi } from '../hooks.js'
import { apiGet, apiPost, apiDelete } from '../api.js'
import { Modal } from '../components/modal.jsx'
import { TextInput, SelectInput } from '../components/form-field.jsx'
import { ConfirmDialog } from '../components/confirm-dialog.jsx'
import { useToast } from '../components/toast.jsx'
import { useAuth } from '../auth.jsx'

export function EscalationPolicyDetail({ id }) {
  const { data: policy, loading } = useApi(`/escalation-policies/${id}`)
  const { data: steps, refetch: refetchSteps } = useApi(`/escalation-policies/${id}/steps`)
  const { data: users } = useApi('/users')
  const { data: schedules } = useApi('/schedules')
  const { user: currentUser } = useAuth()
  const isAdmin = currentUser?.role === 'admin'
  const toast = useToast()

  const [stepModalOpen, setStepModalOpen] = useState(false)
  const [stepForm, setStepForm] = useState({ delay_minutes: '5' })
  const [stepErrors, setStepErrors] = useState({})
  const [deleteStep, setDeleteStep] = useState(null)

  const [targetModalOpen, setTargetModalOpen] = useState(false)
  const [targetStepId, setTargetStepId] = useState(null)
  const [targetForm, setTargetForm] = useState({ target_type: 'user', target_id: '' })
  const [targetErrors, setTargetErrors] = useState({})
  const [deleteTarget, setDeleteTarget] = useState(null)

  // Step targets are fetched per step — we'll track them in state
  const [stepTargets, setStepTargets] = useState({})

  const fetchTargets = async (stepId) => {
    const { data } = await apiGet(`/escalation-policies/${id}/steps/${stepId}/targets`)
    if (data) {
      setStepTargets(prev => ({ ...prev, [stepId]: data }))
    }
  }

  // Load targets for all steps when steps change
  const loadAllTargets = () => {
    if (steps) {
      steps.forEach(s => fetchTargets(s.id))
    }
  }

  // Trigger target load when steps data arrives
  if (steps && steps.length > 0 && Object.keys(stepTargets).length === 0) {
    loadAllTargets()
  }

  const userOptions = (users || []).map(u => ({ value: u.id, label: u.name }))
  const scheduleOptions = (schedules || []).map(s => ({ value: s.id, label: s.name }))

  const targetTypeOptions = [
    { value: 'user', label: 'User' },
    { value: 'schedule', label: 'Schedule' },
  ]

  const resolveTargetName = (type, targetId) => {
    if (type === 'user') {
      const u = (users || []).find(u => u.id === targetId)
      return u ? u.name : targetId
    }
    if (type === 'schedule') {
      const s = (schedules || []).find(s => s.id === targetId)
      return s ? s.name : targetId
    }
    return targetId
  }

  const handleCreateStep = async () => {
    const delay = parseInt(stepForm.delay_minutes, 10)
    if (!delay || delay < 1) {
      setStepErrors({ delay_minutes: 'Must be at least 1 minute' })
      return
    }
    const stepOrder = (steps || []).length + 1
    const { error } = await apiPost(`/escalation-policies/${id}/steps`, { step_order: stepOrder, delay_minutes: delay })
    if (error) {
      toast.error(error)
      return
    }
    toast.success('Step added')
    setStepModalOpen(false)
    setStepForm({ delay_minutes: '5' })
    refetchSteps()
  }

  const handleDeleteStep = async () => {
    const { error } = await apiDelete(`/escalation-policies/${id}/steps/${deleteStep.id}`)
    if (error) {
      toast.error(error)
    } else {
      toast.success('Step deleted')
      setStepTargets(prev => {
        const next = { ...prev }
        delete next[deleteStep.id]
        return next
      })
      refetchSteps()
    }
    setDeleteStep(null)
  }

  const openAddTarget = (stepId) => {
    setTargetStepId(stepId)
    setTargetForm({ target_type: 'user', target_id: '' })
    setTargetErrors({})
    setTargetModalOpen(true)
  }

  const handleCreateTarget = async () => {
    if (!targetForm.target_id) {
      setTargetErrors({ target_id: 'Select a target' })
      return
    }
    const { error } = await apiPost(`/escalation-policies/${id}/steps/${targetStepId}/targets`, targetForm)
    if (error) {
      toast.error(error)
      return
    }
    toast.success('Target added')
    setTargetModalOpen(false)
    fetchTargets(targetStepId)
  }

  const handleDeleteTarget = async () => {
    const { stepId, target } = deleteTarget
    const { error } = await apiDelete(`/escalation-policies/${id}/steps/${stepId}/targets/${target.id}`)
    if (error) {
      toast.error(error)
    } else {
      toast.success('Target removed')
      fetchTargets(stepId)
    }
    setDeleteTarget(null)
  }

  if (loading) return <div class="loading">Loading...</div>
  if (!policy) return <div class="page"><p>Policy not found</p></div>

  return (
    <div class="page">
      <div class="page-header">
        <div>
          <a href="/escalation-policies" class="breadcrumb">Escalation Policies</a>
          <h1>{policy.name}</h1>
        </div>
      </div>

      <div class="detail-grid">
        <div class="detail-card">
          <h3>Details</h3>
          {policy.description && (
            <div class="detail-row">
              <span class="detail-label">Description</span>
              <span class="text-muted">{policy.description}</span>
            </div>
          )}
          <div class="detail-row">
            <span class="detail-label">Repeat</span>
            <span>{policy.repeat}x</span>
          </div>
        </div>

        <div class="detail-card">
          <div class="card-header-row">
            <h3>Escalation Steps</h3>
            {isAdmin && <button class="btn btn-primary btn-sm" onClick={() => { setStepModalOpen(true); setStepErrors({}) }}>
              Add Step
            </button>}
          </div>
          {!steps || steps.length === 0 ? (
            <p class="text-muted">No escalation steps. Add steps to define who gets notified and when.</p>
          ) : (
            <div class="step-list">
              {steps.map((step, i) => (
                <div key={step.id} class="step-item">
                  <div class="step-header">
                    <span class="step-number">Step {i + 1}</span>
                    <span class="text-muted">Wait {step.delay_minutes}m before escalating</span>
                    {isAdmin && (
                      <div class="sub-list-actions">
                        <button class="btn btn-primary btn-sm" onClick={() => openAddTarget(step.id)}>Add Target</button>
                        <button class="btn-icon btn-icon-danger" onClick={() => setDeleteStep(step)} title="Delete step">&times;</button>
                      </div>
                    )}
                  </div>
                  <div class="step-targets">
                    {(stepTargets[step.id] || []).length === 0 ? (
                      <span class="text-muted" style="font-size: 12px">No targets — add a user or schedule</span>
                    ) : (
                      (stepTargets[step.id] || []).map(t => (
                        <div key={t.id} class="target-chip">
                          <span class="source-tag">{t.target_type}</span>
                          <span>{resolveTargetName(t.target_type, t.target_id)}</span>
                          {isAdmin && <button class="btn-icon btn-icon-danger" style="font-size: 14px" onClick={() => setDeleteTarget({ stepId: step.id, target: t })}>&times;</button>}
                        </div>
                      ))
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Add Step Modal */}
      <Modal open={stepModalOpen} onClose={() => setStepModalOpen(false)} title="Add Escalation Step">
        <TextInput label="Delay (minutes)" value={stepForm.delay_minutes} onInput={(e) => setStepForm({ delay_minutes: e.target.value })} error={stepErrors.delay_minutes} type="number" />
        <div class="form-actions">
          <button class="btn btn-secondary" onClick={() => setStepModalOpen(false)}>Cancel</button>
          <button class="btn btn-primary" onClick={handleCreateStep}>Add Step</button>
        </div>
      </Modal>

      {/* Add Target Modal */}
      <Modal open={targetModalOpen} onClose={() => setTargetModalOpen(false)} title="Add Target">
        <SelectInput label="Type" value={targetForm.target_type} onChange={(e) => setTargetForm(prev => ({ ...prev, target_type: e.target.value, target_id: '' }))} options={targetTypeOptions} />
        <SelectInput
          label={targetForm.target_type === 'user' ? 'User' : 'Schedule'}
          value={targetForm.target_id}
          onChange={(e) => setTargetForm(prev => ({ ...prev, target_id: e.target.value }))}
          options={targetForm.target_type === 'user' ? userOptions : scheduleOptions}
          placeholder={`Select ${targetForm.target_type}...`}
          error={targetErrors.target_id}
        />
        <div class="form-actions">
          <button class="btn btn-secondary" onClick={() => setTargetModalOpen(false)}>Cancel</button>
          <button class="btn btn-primary" onClick={handleCreateTarget}>Add Target</button>
        </div>
      </Modal>

      {/* Delete confirmations */}
      <ConfirmDialog
        open={!!deleteStep}
        onClose={() => setDeleteStep(null)}
        onConfirm={handleDeleteStep}
        title="Delete Step"
        message="Are you sure you want to delete this escalation step and all its targets?"
      />
      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDeleteTarget}
        title="Remove Target"
        message="Are you sure you want to remove this target from the escalation step?"
      />
    </div>
  )
}
