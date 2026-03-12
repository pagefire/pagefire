import { useState } from 'preact/hooks'
import { useApi } from '../hooks.js'
import { apiGet, apiPost, apiDelete } from '../api.js'
import { Modal } from '../components/modal.jsx'
import { TextInput, SelectInput } from '../components/form-field.jsx'
import { ConfirmDialog } from '../components/confirm-dialog.jsx'
import { useToast } from '../components/toast.jsx'
import { useAuth } from '../auth.jsx'
import { TimeAgo } from '../components/time-ago.jsx'

export function EscalationPolicyDetail({ id }) {
  const { data: policy, loading } = useApi(`/escalation-policies/${id}`)
  const { data: steps, refetch: refetchSteps } = useApi(`/escalation-policies/${id}/steps`)
  const { data: users } = useApi('/users')
  const { data: schedules } = useApi('/schedules')
  const { data: services } = useApi('/services')
  const { user: currentUser } = useAuth()
  const isAdmin = currentUser?.role === 'admin'
  const toast = useToast()

  const [stepModalOpen, setStepModalOpen] = useState(false)
  const [stepForm, setStepForm] = useState({ delay_minutes: '0' })
  const [stepErrors, setStepErrors] = useState({})
  const [deleteStep, setDeleteStep] = useState(null)

  const [targetModalOpen, setTargetModalOpen] = useState(false)
  const [targetStepId, setTargetStepId] = useState(null)
  const [targetForm, setTargetForm] = useState({ target_type: 'user', target_id: '' })
  const [targetErrors, setTargetErrors] = useState({})
  const [deleteTarget, setDeleteTarget] = useState(null)

  const [stepTargets, setStepTargets] = useState({})

  const fetchTargets = async (stepId) => {
    const { data } = await apiGet(`/escalation-policies/${id}/steps/${stepId}/targets`)
    if (data) {
      setStepTargets(prev => ({ ...prev, [stepId]: data }))
    }
  }

  const loadAllTargets = () => {
    if (steps) {
      steps.forEach(s => fetchTargets(s.id))
    }
  }

  if (steps && steps.length > 0 && Object.keys(stepTargets).length === 0) {
    loadAllTargets()
  }

  const userOptions = (users || []).map(u => ({ value: u.id, label: u.name }))
  const scheduleOptions = (schedules || []).map(s => ({ value: s.id, label: s.name }))

  const targetTypeOptions = [
    { value: 'user', label: 'User' },
    { value: 'schedule', label: 'On-Call Schedule' },
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

  // Services using this policy
  const linkedServices = (services || []).filter(s => s.escalation_policy_id === id)

  const handleCreateStep = async () => {
    const delay = parseInt(stepForm.delay_minutes, 10)
    if (isNaN(delay) || delay < 0) {
      setStepErrors({ delay_minutes: 'Must be 0 or more minutes' })
      return
    }
    const stepNumber = (steps || []).length
    const { error } = await apiPost(`/escalation-policies/${id}/steps`, { step_number: stepNumber, delay_minutes: delay })
    if (error) {
      toast.error(error)
      return
    }
    toast.success('Step added')
    setStepModalOpen(false)
    setStepForm({ delay_minutes: '0' })
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

  // Build a human-readable summary of the escalation flow
  const sortedSteps = [...(steps || [])].sort((a, b) => a.step_number - b.step_number)
  const totalCycles = 1 + (policy.repeat || 0)

  return (
    <div class="page">
      <div class="page-header">
        <div>
          <a href="/escalation-policies" class="breadcrumb">Escalation Policies</a>
          <h1>{policy.name}</h1>
        </div>
      </div>

      {/* How It Works summary */}
      {sortedSteps.length > 0 && (
        <div class="detail-card" style="margin-bottom: 20px">
          <h3>How It Works</h3>
          <p class="text-muted" style="margin-bottom: 12px; font-size: 13px">
            When an alert is triggered on a service using this policy, PageFire notifies targets in order.
            If no one acknowledges, it moves to the next step after the configured delay.
            {policy.repeat > 0
              ? ` The entire sequence repeats ${policy.repeat} time${policy.repeat > 1 ? 's' : ''} (${totalCycles} total cycles) before giving up.`
              : ' The sequence runs once with no repeats.'}
          </p>
          <div class="escalation-flow">
            {sortedSteps.map((step, i) => {
              const targets = stepTargets[step.id] || []
              return (
                <div key={step.id} class="flow-step">
                  <div class="flow-step-marker">
                    <span class="flow-step-number">{i + 1}</span>
                    {i < sortedSteps.length - 1 && <div class="flow-step-line" />}
                  </div>
                  <div class="flow-step-content">
                    <div class="flow-step-title">
                      {step.delay_minutes === 0
                        ? (i === 0 ? 'Immediately notify' : 'Then immediately notify')
                        : `After ${step.delay_minutes} min, notify`}
                    </div>
                    <div class="flow-step-targets">
                      {targets.length === 0 ? (
                        <span class="text-muted">No targets configured</span>
                      ) : (
                        targets.map(t => (
                          <span key={t.id} class="target-chip">
                            <span class="source-tag">{t.target_type === 'schedule' ? 'schedule' : 'user'}</span>
                            {resolveTargetName(t.target_type, t.target_id)}
                          </span>
                        ))
                      )}
                    </div>
                  </div>
                </div>
              )
            })}
            {policy.repeat > 0 && (
              <div class="flow-step">
                <div class="flow-step-marker">
                  <span class="flow-step-number">&#8635;</span>
                </div>
                <div class="flow-step-content">
                  <div class="flow-step-title text-muted">
                    Repeat from Step 1 ({policy.repeat}x)
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>
      )}

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
            <span class="detail-label">Repeat Cycles</span>
            <span>{policy.repeat === 0 ? 'None — run once' : `${policy.repeat}x (${totalCycles} total cycles)`}</span>
          </div>
          <div class="detail-row">
            <span class="detail-label">Steps</span>
            <span>{sortedSteps.length}</span>
          </div>
          <div class="detail-row">
            <span class="detail-label">Created</span>
            <TimeAgo time={policy.created_at} />
          </div>
          {linkedServices.length > 0 && (
            <div class="detail-row">
              <span class="detail-label">Used By</span>
              <span>{linkedServices.map(s => s.name).join(', ')}</span>
            </div>
          )}
        </div>

        <div class="detail-card">
          <div class="card-header-row">
            <h3>Escalation Steps</h3>
            {isAdmin && <button class="btn btn-primary btn-sm" onClick={() => { setStepModalOpen(true); setStepErrors({}); setStepForm({ delay_minutes: '0' }) }}>
              Add Step
            </button>}
          </div>
          {sortedSteps.length === 0 ? (
            <p class="text-muted">No escalation steps. Add steps to define who gets notified and when.</p>
          ) : (
            <div class="step-list">
              {sortedSteps.map((step, i) => (
                <div key={step.id} class="step-item">
                  <div class="step-header">
                    <span class="step-number">Step {i + 1}</span>
                    <span class="text-muted">
                      {step.delay_minutes === 0 ? 'Notify immediately' : `Wait ${step.delay_minutes} min before notifying`}
                    </span>
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
        <p class="text-muted" style="margin-bottom: 12px; font-size: 13px">
          Configure how long to wait before notifying this step's targets. Use 0 for immediate notification.
        </p>
        <TextInput
          label="Delay (minutes)"
          value={stepForm.delay_minutes}
          onInput={(e) => setStepForm({ delay_minutes: e.target.value })}
          error={stepErrors.delay_minutes}
          type="number"
          placeholder="0"
        />
        <p class="text-muted" style="font-size: 12px; margin-top: 4px">
          0 = notify immediately, 5 = wait 5 minutes after alert triggers (or after previous step)
        </p>
        <div class="form-actions">
          <button class="btn btn-secondary" onClick={() => setStepModalOpen(false)}>Cancel</button>
          <button class="btn btn-primary" onClick={handleCreateStep}>Add Step</button>
        </div>
      </Modal>

      {/* Add Target Modal */}
      <Modal open={targetModalOpen} onClose={() => setTargetModalOpen(false)} title="Add Target">
        <SelectInput label="Type" value={targetForm.target_type} onChange={(e) => setTargetForm(prev => ({ ...prev, target_type: e.target.value, target_id: '' }))} options={targetTypeOptions} />
        <SelectInput
          label={targetForm.target_type === 'user' ? 'User' : 'On-Call Schedule'}
          value={targetForm.target_id}
          onChange={(e) => setTargetForm(prev => ({ ...prev, target_id: e.target.value }))}
          options={targetForm.target_type === 'user' ? userOptions : scheduleOptions}
          placeholder={`Select ${targetForm.target_type === 'schedule' ? 'schedule' : 'user'}...`}
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
